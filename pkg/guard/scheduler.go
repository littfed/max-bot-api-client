package guard

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// ScheduledPost represents a rich message queued for timed publication.
type ScheduledPost struct {
	ID          string
	ChatID      int64
	Text        string
	Format      string // "markdown" or "html"
	Attachments []maxbot.Attachment
	Keyboard    *maxbot.InlineKeyboard
	PublishAt   time.Time
}

// PostScheduler manages the background queue and timed dispatch of channel/group posts.
type PostScheduler struct {
	client    *maxbot.Client
	posts     map[string]*ScheduledPost
	timers    map[string]*time.Timer
	onSuccess func(ctx context.Context, post *ScheduledPost, mid string)
	onError   func(ctx context.Context, post *ScheduledPost, err error)
	mu        sync.Mutex
	stopCh    chan struct{}
}

// SchedulerOption configures PostScheduler.
type SchedulerOption func(*PostScheduler)

// WithOnPostSuccess registers a callback executed when a scheduled post is successfully published.
func WithOnPostSuccess(fn func(ctx context.Context, post *ScheduledPost, mid string)) SchedulerOption {
	return func(s *PostScheduler) {
		s.onSuccess = fn
	}
}

// WithOnPostError registers a callback executed when a scheduled post publication fails.
func WithOnPostError(fn func(ctx context.Context, post *ScheduledPost, err error)) SchedulerOption {
	return func(s *PostScheduler) {
		s.onError = fn
	}
}

// NewPostScheduler creates a new PostScheduler instance.
func NewPostScheduler(client *maxbot.Client, opts ...SchedulerOption) *PostScheduler {
	s := &PostScheduler{
		client: client,
		posts:  make(map[string]*ScheduledPost),
		timers: make(map[string]*time.Timer),
		stopCh: make(chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Schedule queues a post for publication at post.PublishAt.
func (s *PostScheduler) Schedule(post ScheduledPost) error {
	if post.ID == "" {
		return errors.New("guard: post.ID cannot be empty")
	}
	if post.ChatID == 0 {
		return errors.New("guard: post.ChatID cannot be 0")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Cancel existing if replacing
	if t, ok := s.timers[post.ID]; ok {
		t.Stop()
		delete(s.timers, post.ID)
	}

	pCopy := post
	s.posts[post.ID] = &pCopy

	delay := time.Until(post.PublishAt)
	if delay < 0 {
		delay = 0
	}

	s.timers[post.ID] = time.AfterFunc(delay, func() {
		s.dispatch(post.ID)
	})

	return nil
}

// Cancel removes a scheduled post from the queue.
func (s *PostScheduler) Cancel(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.timers[id]
	if !ok {
		return false
	}
	t.Stop()
	delete(s.timers, id)
	delete(s.posts, id)
	return true
}

// ListPending returns a snapshot of all currently queued scheduled posts.
func (s *PostScheduler) ListPending() []ScheduledPost {
	s.mu.Lock()
	defer s.mu.Unlock()

	list := make([]ScheduledPost, 0, len(s.posts))
	for _, p := range s.posts {
		list = append(list, *p)
	}
	return list
}

func (s *PostScheduler) dispatch(id string) {
	s.mu.Lock()
	post, ok := s.posts[id]
	if !ok {
		s.mu.Unlock()
		return
	}
	delete(s.posts, id)
	delete(s.timers, id)
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if s.client == nil {
		if s.onSuccess != nil {
			s.onSuccess(ctx, post, "mock_mid_"+id)
		}
		return
	}

	format := post.Format
	if format == "" {
		format = "markdown"
	}

	req := &maxbot.SendMessageRequest{
		Text:        post.Text,
		Format:      format,
		Attachments: post.Attachments,
	}
	if post.Keyboard != nil {
		req.Attachments = append(req.Attachments, post.Keyboard.ToAttachment())
	}

	resp, err := s.client.SendMessage(ctx, maxbot.ToChat(post.ChatID), req)
	if err != nil {
		if s.onError != nil {
			s.onError(ctx, post, err)
		}
		return
	}

	mid := ""
	if resp != nil {
		mid = resp.ID()
	}

	if s.onSuccess != nil {
		s.onSuccess(ctx, post, mid)
	}
}

// Stop cancels all pending timers and stops the scheduler.
func (s *PostScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, t := range s.timers {
		t.Stop()
	}
	s.timers = make(map[string]*time.Timer)
	s.posts = make(map[string]*ScheduledPost)
}

package store

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// StoreState is the serialized snapshot of all dynamic application data.
type StoreState struct {
	Channels  map[int64]Channel    `json:"channels"`
	Orders    map[string]Order     `json:"orders"`
	Reactions map[string]*Reaction `json:"reactions"`
	Polls     map[string]*Poll     `json:"polls"`
	Giveaways map[string]*Giveaway `json:"giveaways"`
}

// Store provides thread-safe access and persistence for channels, orders, reactions, and polls.
type Store struct {
	mu        sync.RWMutex
	filePath  string
	channels  map[int64]Channel
	orders    map[string]Order
	reactions map[string]*Reaction
	polls     map[string]*Poll
	giveaways map[string]*Giveaway
}

// New initializes or loads a Store from the specified JSON file path.
func New(filePath string) (*Store, error) {
	s := &Store{
		filePath:  filePath,
		channels:  make(map[int64]Channel),
		orders:    make(map[string]Order),
		reactions: make(map[string]*Reaction),
		polls:     make(map[string]*Poll),
		giveaways: make(map[string]*Giveaway),
	}

	if filePath != "" {
		if err := s.load(); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load store from %s: %w", filePath, err)
		}
	}

	// Seed known channels if store is empty
	s.seedDefaultChannels()

	return s, nil
}

func (s *Store) seedDefaultChannels() {
	s.mu.Lock()
	defer s.mu.Unlock()

	defaultPerms := []string{"write", "edit", "delete", "pin_message", "add_remove_members", "change_chat_info", "read_all_messages"}
	now := time.Now()

	if _, ok := s.channels[-78368186601503]; !ok {
		s.channels[-78368186601503] = Channel{
			ChatID:      -78368186601503,
			Title:       "Тест",
			Type:        "channel",
			IsAdmin:     true,
			Permissions: defaultPerms,
			LastUpdated: now,
		}
	}
	if _, ok := s.channels[-78548596658207]; !ok {
		s.channels[-78548596658207] = Channel{
			ChatID:      -78548596658207,
			Title:       "littfed",
			Type:        "channel",
			IsAdmin:     true,
			Permissions: defaultPerms,
			LastUpdated: now,
		}
	}
}

// load reads and deserializes the JSON state file.
func (s *Store) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var state StoreState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if state.Channels != nil {
		s.channels = state.Channels
	}
	if state.Orders != nil {
		s.orders = state.Orders
	}
	if state.Reactions != nil {
		s.reactions = state.Reactions
	}
	if state.Polls != nil {
		s.polls = state.Polls
	}
	if state.Giveaways != nil {
		s.giveaways = state.Giveaways
	}

	return nil
}

// saveLocked writes the state atomically to disk.
func (s *Store) saveLocked() error {
	if s.filePath == "" {
		return nil
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	state := StoreState{
		Channels:  s.channels,
		Orders:    s.orders,
		Reactions: s.reactions,
		Polls:     s.polls,
		Giveaways: s.giveaways,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal store state: %w", err)
	}

	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write tmp store file: %w", err)
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		return fmt.Errorf("failed to replace store file: %w", err)
	}

	return nil
}

// --- Channel Operations ---

func (s *Store) SaveChannel(ch Channel) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch.LastUpdated = time.Now()
	s.channels[ch.ChatID] = ch
	return s.saveLocked()
}

func (s *Store) GetChannel(chatID int64) (Channel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ch, ok := s.channels[chatID]
	return ch, ok
}

func (s *Store) ListChannels() []Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]Channel, 0, len(s.channels))
	for _, ch := range s.channels {
		list = append(list, ch)
	}
	return list
}

// --- Order Operations ---

func (s *Store) CreateOrder(order Order) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if order.CreatedAt.IsZero() {
		order.CreatedAt = time.Now()
	}
	if order.Status == "" {
		order.Status = "accepted"
	}
	s.orders[order.ID] = order
	return s.saveLocked()
}

func (s *Store) GetOrder(id string) (Order, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	order, ok := s.orders[id]
	return order, ok
}

func (s *Store) ListOrdersByUser(userID int64) []Order {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var userOrders []Order
	for _, ord := range s.orders {
		if ord.UserID == userID {
			userOrders = append(userOrders, ord)
		}
	}
	return userOrders
}

func (s *Store) ListAllOrders() []Order {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := make([]Order, 0, len(s.orders))
	for _, ord := range s.orders {
		all = append(all, ord)
	}
	return all
}

func (s *Store) UpdateOrderStatus(id string, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ord, ok := s.orders[id]
	if !ok {
		return fmt.Errorf("order %s not found", id)
	}
	ord.Status = status
	s.orders[id] = ord
	return s.saveLocked()
}

// --- Reaction Operations ---

// ToggleReaction handles user click on a post reaction emoji.
// It switches reactions if the user already clicked a different emoji,
// or removes it if clicking the same emoji again.
// Returns the updated count map for that post.
func (s *Store) ToggleReaction(postMID string, chatID int64, userID int64, emoji string) (map[string]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rx, ok := s.reactions[postMID]
	if !ok {
		rx = &Reaction{
			PostMID:   postMID,
			ChatID:    chatID,
			Counts:    map[string]int{"👍": 0, "🔥": 0, "❤️": 0},
			UserVotes: make(map[int64]string),
		}
		s.reactions[postMID] = rx
	}

	prevEmoji, hasVoted := rx.UserVotes[userID]
	if hasVoted && prevEmoji == emoji {
		// User clicks same emoji -> retract vote
		delete(rx.UserVotes, userID)
		if rx.Counts[emoji] > 0 {
			rx.Counts[emoji]--
		}
	} else {
		// If voted something else before, decrement old
		if hasVoted && rx.Counts[prevEmoji] > 0 {
			rx.Counts[prevEmoji]--
		}
		// Increment new emoji
		rx.UserVotes[userID] = emoji
		rx.Counts[emoji]++
	}

	// Copy counts to return safely
	countsCopy := make(map[string]int, len(rx.Counts))
	for k, v := range rx.Counts {
		countsCopy[k] = v
	}

	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return countsCopy, nil
}

func (s *Store) GetReactions(postMID string) (map[string]int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rx, ok := s.reactions[postMID]
	if !ok {
		return nil, false
	}
	countsCopy := make(map[string]int, len(rx.Counts))
	for k, v := range rx.Counts {
		countsCopy[k] = v
	}
	return countsCopy, true
}

// --- Poll Operations ---

func (s *Store) CreatePoll(poll Poll) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if poll.CreatedAt.IsZero() {
		poll.CreatedAt = time.Now()
	}
	if poll.Votes == nil {
		poll.Votes = make(map[int64]int)
	}
	s.polls[poll.ID] = &poll
	return s.saveLocked()
}

func (s *Store) GetPoll(id string) (Poll, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	poll, ok := s.polls[id]
	if !ok {
		return Poll{}, false
	}
	return *poll, true
}

// VotePoll records or changes a user's vote and calculates updated option percentages.
func (s *Store) VotePoll(pollID string, userID int64, optionIdx int) ([]int, []int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.polls[pollID]
	if !ok {
		return nil, nil, fmt.Errorf("poll %s not found", pollID)
	}
	if optionIdx < 0 || optionIdx >= len(p.Options) {
		return nil, nil, fmt.Errorf("invalid option index %d", optionIdx)
	}

	p.Votes[userID] = optionIdx

	// Calculate counts and percentages
	counts := make([]int, len(p.Options))
	totalVotes := len(p.Votes)
	for _, opt := range p.Votes {
		if opt >= 0 && opt < len(counts) {
			counts[opt]++
		}
	}

	percentages := make([]int, len(p.Options))
	if totalVotes > 0 {
		for i, cnt := range counts {
			percentages[i] = int(float64(cnt) / float64(totalVotes) * 100)
		}
	}

	if err := s.saveLocked(); err != nil {
		return nil, nil, err
	}

	return counts, percentages, nil
}

// --- Giveaway Operations ---

func (s *Store) CreateGiveaway(gw Giveaway) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if gw.CreatedAt.IsZero() {
		gw.CreatedAt = time.Now()
	}
	if gw.Participants == nil {
		gw.Participants = make(map[int64]string)
	}
	gw.Active = true
	s.giveaways[gw.ID] = &gw
	return s.saveLocked()
}

func (s *Store) GetGiveaway(id string) (Giveaway, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	gw, ok := s.giveaways[id]
	if !ok {
		return Giveaway{}, false
	}
	return *gw, true
}

func (s *Store) AddGiveawayParticipant(id string, userID int64, userName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	gw, ok := s.giveaways[id]
	if !ok {
		return fmt.Errorf("giveaway %s not found", id)
	}
	if !gw.Active {
		return fmt.Errorf("giveaway %s is closed", id)
	}
	gw.Participants[userID] = userName
	return s.saveLocked()
}

func (s *Store) DrawGiveawayWinners(id string, count int) ([]int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	gw, ok := s.giveaways[id]
	if !ok {
		return nil, fmt.Errorf("giveaway %s not found", id)
	}

	var uids []int64
	for uid := range gw.Participants {
		uids = append(uids, uid)
	}

	if len(uids) == 0 {
		return nil, fmt.Errorf("no participants in giveaway %s", id)
	}

	// Shuffle
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(uids), func(i, j int) {
		uids[i], uids[j] = uids[j], uids[i]
	})

	if count > len(uids) {
		count = len(uids)
	}
	winners := uids[:count]
	gw.Winners = winners
	gw.Active = false

	if err := s.saveLocked(); err != nil {
		return nil, err
	}

	return winners, nil
}

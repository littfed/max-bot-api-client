package livetest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// ErrMatcherTimeout is returned when an expected update does not arrive before the deadline.
var ErrMatcherTimeout = errors.New("timeout waiting for matching update")

// UpdatePredicate checks whether an update matches the expected criteria.
type UpdatePredicate func(u *maxbot.Update) bool

// Matcher provides polling-based waiting for specific inbound updates.
type Matcher struct {
	client *maxbot.Client
	marker int64
}

// NewMatcher initializes an update matcher and synchronizes the latest marker.
func NewMatcher(ctx context.Context, client *maxbot.Client) (*Matcher, error) {
	m := &Matcher{
		client: client,
	}
	if err := m.SyncLatestMarker(ctx); err != nil {
		return nil, err
	}
	return m, nil
}

// SyncLatestMarker fetches the latest marker from the server so we only match fresh updates.
func (m *Matcher) SyncLatestMarker(ctx context.Context) error {
	resp, err := m.client.GetUpdates(ctx, &maxbot.GetUpdatesOptions{
		Limit:   1,
		Timeout: 0,
	})
	if err != nil {
		return fmt.Errorf("failed to sync updates marker: %w", err)
	}
	m.marker = resp.Marker
	return nil
}

// WaitFor blocks until a matching update arrives or ctx is cancelled.
// It returns the matching update and the elapsed wait duration (RTT).
func (m *Matcher) WaitFor(ctx context.Context, pred UpdatePredicate) (*maxbot.Update, time.Duration, error) {
	start := time.Now()

	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, time.Since(start), ErrMatcherTimeout
			}
			return nil, time.Since(start), ctx.Err()
		default:
		}

		// Poll with 3-second server long poll timeout
		pollCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		resp, err := m.client.GetUpdates(pollCtx, &maxbot.GetUpdatesOptions{
			Marker:  m.marker,
			Limit:   50,
			Timeout: 3,
		})
		cancel()

		if err != nil {
			// Check if outer context died
			if ctx.Err() != nil {
				return nil, time.Since(start), ErrMatcherTimeout
			}
			// Transient poll error, pause and retry
			select {
			case <-time.After(300 * time.Millisecond):
			case <-ctx.Done():
				return nil, time.Since(start), ErrMatcherTimeout
			}
			continue
		}

		if resp != nil {
			m.marker = resp.Marker

			for _, update := range resp.Updates {
				up := update
				if pred(&up) {
					return &up, time.Since(start), nil
				}
			}
		}
	}
}

// CallbackPayloadMatcher matches an update of type message_callback with matching payload and optional userID.
func CallbackPayloadMatcher(expectedPayload string, targetUserID int64) UpdatePredicate {
	return func(u *maxbot.Update) bool {
		if u.UpdateType != maxbot.UpdateTypeMessageCallback || u.Callback == nil {
			return false
		}
		if u.Callback.Payload != expectedPayload {
			return false
		}
		if targetUserID != 0 && u.Callback.User != nil && u.Callback.User.UserID != targetUserID {
			return false
		}
		return true
	}
}

// TextMessageMatcher matches an incoming text message with exact text and optional sender userID.
func TextMessageMatcher(expectedText string, targetUserID int64) UpdatePredicate {
	return func(u *maxbot.Update) bool {
		if u.UpdateType != maxbot.UpdateTypeMessageCreated || u.Message == nil {
			return false
		}
		if u.Message.Body.Text != expectedText {
			return false
		}
		if targetUserID != 0 && u.Message.Sender.UserID != targetUserID {
			return false
		}
		return true
	}
}

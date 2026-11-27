package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/littfed/max-bot-api-client/pkg/db"
	_ "modernc.org/sqlite"
)

// SQLStore implements a persistent database-backed storage for PostgreSQL and SQLite.
// SQLStore is safe for concurrent use by multiple goroutines; an internal mutex
// serializes operations to prevent SQLITE_BUSY contention on SQLite.
type SQLStore struct {
	db      *sql.DB
	dialect string
	mu      sync.RWMutex
}

// NewSQLStore creates a new SQLStore and runs embedded Goose migrations for the target dialect.
func NewSQLStore(database *sql.DB, dialect string) (*SQLStore, error) {
	if database == nil {
		return nil, errors.New("store: database connection cannot be nil")
	}

	normDialect := strings.ToLower(strings.TrimSpace(dialect))
	if normDialect == "sqlite" {
		normDialect = "sqlite3"
	} else if normDialect == "postgresql" || normDialect == "pgx" {
		normDialect = "postgres"
	}

	// 1. Run migrations automatically on boot
	if err := db.RunMigrations(database, normDialect); err != nil {
		return nil, fmt.Errorf("store: failed to run migrations for %s: %w", normDialect, err)
	}

	s := &SQLStore{
		db:      database,
		dialect: normDialect,
	}

	// 2. Seed default channels if empty
	s.seedDefaultChannels()

	return s, nil
}

func (s *SQLStore) seedDefaultChannels() {
	defaultPerms := []string{"write", "edit", "delete", "pin_message", "add_remove_members", "change_chat_info", "read_all_messages"}
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	channels := []Channel{
		{
			ChatID:      -78368186601503,
			Title:       "Тест",
			Type:        "channel",
			IsAdmin:     true,
			Permissions: defaultPerms,
			LastUpdated: now,
		},
		{
			ChatID:      -78548596658207,
			Title:       "littfed",
			Type:        "channel",
			IsAdmin:     true,
			Permissions: defaultPerms,
			LastUpdated: now,
		},
	}

	for _, ch := range channels {
		if _, exists := s.getChannelLocked(context.Background(), ch.ChatID); !exists {
			_ = s.saveChannelLocked(context.Background(), ch)
		}
	}
}

// --- Channel Operations ---

func (s *SQLStore) SaveChannel(ch Channel) error {
	return s.SaveChannelContext(context.Background(), ch)
}

func (s *SQLStore) SaveChannelContext(ctx context.Context, ch Channel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveChannelLocked(ctx, ch)
}

func (s *SQLStore) saveChannelLocked(ctx context.Context, ch Channel) error {
	permsJSON, err := json.Marshal(ch.Permissions)
	if err != nil {
		permsJSON = []byte("[]")
	}

	if ch.LastUpdated.IsZero() {
		ch.LastUpdated = time.Now()
	}

	query := `INSERT INTO channels (chat_id, title, type, is_admin, permissions, last_updated)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (chat_id) DO UPDATE SET
	title = EXCLUDED.title,
	type = EXCLUDED.type,
	is_admin = EXCLUDED.is_admin,
	permissions = EXCLUDED.permissions,
	last_updated = EXCLUDED.last_updated;`

	_, err = s.db.ExecContext(ctx, query, ch.ChatID, ch.Title, ch.Type, ch.IsAdmin, string(permsJSON), ch.LastUpdated)
	return err
}

func (s *SQLStore) GetChannel(chatID int64) (Channel, bool) {
	return s.GetChannelContext(context.Background(), chatID)
}

func (s *SQLStore) GetChannelContext(ctx context.Context, chatID int64) (Channel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getChannelLocked(ctx, chatID)
}

func (s *SQLStore) getChannelLocked(ctx context.Context, chatID int64) (Channel, bool) {
	query := `SELECT chat_id, title, type, is_admin, permissions, last_updated FROM channels WHERE chat_id = $1;`
	row := s.db.QueryRowContext(ctx, query, chatID)

	var ch Channel
	var permsStr string
	err := row.Scan(&ch.ChatID, &ch.Title, &ch.Type, &ch.IsAdmin, &permsStr, &ch.LastUpdated)
	if err != nil {
		return Channel{}, false
	}
	_ = json.Unmarshal([]byte(permsStr), &ch.Permissions)
	return ch, true
}

func (s *SQLStore) ListChannels() []Channel {
	return s.ListChannelsContext(context.Background())
}

func (s *SQLStore) ListChannelsContext(ctx context.Context) []Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT chat_id, title, type, is_admin, permissions, last_updated FROM channels ORDER BY title ASC;`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []Channel
	for rows.Next() {
		var ch Channel
		var permsStr string
		if err := rows.Scan(&ch.ChatID, &ch.Title, &ch.Type, &ch.IsAdmin, &permsStr, &ch.LastUpdated); err == nil {
			_ = json.Unmarshal([]byte(permsStr), &ch.Permissions)
			result = append(result, ch)
		}
	}
	return result
}

// --- Order Operations ---

func (s *SQLStore) CreateOrder(order Order) error {
	return s.CreateOrderContext(context.Background(), order)
}

func (s *SQLStore) CreateOrderContext(ctx context.Context, order Order) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if order.CreatedAt.IsZero() {
		order.CreatedAt = time.Now()
	}
	if order.Status == "" {
		order.Status = "accepted"
	}
	if order.DeliveryType == "" {
		order.DeliveryType = "delivery"
	}

	itemsJSON, err := json.Marshal(order.Items)
	if err != nil {
		itemsJSON = []byte("[]")
	}

	query := `INSERT INTO orders (id, user_id, user_name, items, total, delivery_type, address, status, verified, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (id) DO UPDATE SET
	status = EXCLUDED.status,
	address = EXCLUDED.address;`

	_, err = s.db.ExecContext(ctx, query, order.ID, order.UserID, order.UserName, string(itemsJSON), order.Total, order.DeliveryType, order.Address, order.Status, order.Verified, order.CreatedAt)
	return err
}

func (s *SQLStore) GetOrder(id string) (Order, bool) {
	return s.GetOrderContext(context.Background(), id)
}

func (s *SQLStore) GetOrderContext(ctx context.Context, id string) (Order, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, user_id, user_name, items, total, delivery_type, address, status, verified, created_at FROM orders WHERE id = $1;`
	row := s.db.QueryRowContext(ctx, query, id)

	var ord Order
	var itemsStr string
	err := row.Scan(&ord.ID, &ord.UserID, &ord.UserName, &itemsStr, &ord.Total, &ord.DeliveryType, &ord.Address, &ord.Status, &ord.Verified, &ord.CreatedAt)
	if err != nil {
		return Order{}, false
	}
	_ = json.Unmarshal([]byte(itemsStr), &ord.Items)
	return ord, true
}

func (s *SQLStore) ListOrdersByUser(userID int64) []Order {
	return s.ListOrdersByUserContext(context.Background(), userID)
}

func (s *SQLStore) ListOrdersByUserContext(ctx context.Context, userID int64) []Order {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, user_id, user_name, items, total, delivery_type, address, status, verified, created_at FROM orders WHERE user_id = $1 ORDER BY created_at DESC;`
	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []Order
	for rows.Next() {
		var ord Order
		var itemsStr string
		if err := rows.Scan(&ord.ID, &ord.UserID, &ord.UserName, &itemsStr, &ord.Total, &ord.DeliveryType, &ord.Address, &ord.Status, &ord.Verified, &ord.CreatedAt); err == nil {
			_ = json.Unmarshal([]byte(itemsStr), &ord.Items)
			result = append(result, ord)
		}
	}
	return result
}

func (s *SQLStore) ListAllOrders() []Order {
	return s.ListAllOrdersContext(context.Background())
}

func (s *SQLStore) ListAllOrdersContext(ctx context.Context) []Order {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, user_id, user_name, items, total, delivery_type, address, status, verified, created_at FROM orders ORDER BY created_at DESC;`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []Order
	for rows.Next() {
		var ord Order
		var itemsStr string
		if err := rows.Scan(&ord.ID, &ord.UserID, &ord.UserName, &itemsStr, &ord.Total, &ord.DeliveryType, &ord.Address, &ord.Status, &ord.Verified, &ord.CreatedAt); err == nil {
			_ = json.Unmarshal([]byte(itemsStr), &ord.Items)
			result = append(result, ord)
		}
	}
	return result
}

func (s *SQLStore) UpdateOrderStatus(id string, status string) error {
	return s.UpdateOrderStatusContext(context.Background(), id, status)
}

func (s *SQLStore) UpdateOrderStatusContext(ctx context.Context, id string, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `UPDATE orders SET status = $1 WHERE id = $2;`
	res, err := s.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return err
	}
	rowsAff, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAff == 0 {
		return fmt.Errorf("order %q not found", id)
	}
	return nil
}

// --- Reaction Operations ---

func (s *SQLStore) ToggleReaction(postMID string, chatID int64, userID int64, emoji string) (map[string]int, error) {
	return s.ToggleReactionContext(context.Background(), postMID, chatID, userID, emoji)
}

func (s *SQLStore) ToggleReactionContext(ctx context.Context, postMID string, chatID int64, userID int64, emoji string) (map[string]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Fetch current reaction state
	query := `SELECT counts, user_votes FROM reactions WHERE post_mid = $1;`
	if s.dialect == "postgres" {
		query = `SELECT counts, user_votes FROM reactions WHERE post_mid = $1 FOR UPDATE;`
	}
	row := tx.QueryRowContext(ctx, query, postMID)

	counts := make(map[string]int)
	userVotes := make(map[int64]string)

	var countsStr, votesStr string
	err = row.Scan(&countsStr, &votesStr)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		_ = json.Unmarshal([]byte(countsStr), &counts)
		_ = json.Unmarshal([]byte(votesStr), &userVotes)
	}

	// 2. Toggle reaction
	if emoji != "" {
		prevEmoji, hadVoted := userVotes[userID]
		if hadVoted && prevEmoji == emoji {
			delete(userVotes, userID)
			counts[emoji]--
			if counts[emoji] <= 0 {
				delete(counts, emoji)
			}
		} else {
			if hadVoted {
				counts[prevEmoji]--
				if counts[prevEmoji] <= 0 {
					delete(counts, prevEmoji)
				}
			}
			userVotes[userID] = emoji
			counts[emoji]++
		}
	}

	// 3. Save state
	cJSON, _ := json.Marshal(counts)
	vJSON, _ := json.Marshal(userVotes)

	saveQuery := `INSERT INTO reactions (post_mid, chat_id, counts, user_votes)
VALUES ($1, $2, $3, $4)
ON CONFLICT (post_mid) DO UPDATE SET
	counts = EXCLUDED.counts,
	user_votes = EXCLUDED.user_votes;`

	if _, err := tx.ExecContext(ctx, saveQuery, postMID, chatID, string(cJSON), string(vJSON)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	resCounts := make(map[string]int, len(counts))
	for k, v := range counts {
		resCounts[k] = v
	}
	return resCounts, nil
}

func (s *SQLStore) GetReactions(postMID string) (map[string]int, bool) {
	return s.GetReactionsContext(context.Background(), postMID)
}

func (s *SQLStore) GetReactionsContext(ctx context.Context, postMID string) (map[string]int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT counts FROM reactions WHERE post_mid = $1;`
	row := s.db.QueryRowContext(ctx, query, postMID)

	var countsStr string
	if err := row.Scan(&countsStr); err != nil {
		return nil, false
	}
	counts := make(map[string]int)
	_ = json.Unmarshal([]byte(countsStr), &counts)
	return counts, true
}

// --- Poll Operations ---

func (s *SQLStore) CreatePoll(poll Poll) error {
	return s.CreatePollContext(context.Background(), poll)
}

func (s *SQLStore) CreatePollContext(ctx context.Context, poll Poll) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if poll.CreatedAt.IsZero() {
		poll.CreatedAt = time.Now()
	}
	if poll.Votes == nil {
		poll.Votes = make(map[int64]int)
	}

	optsJSON, _ := json.Marshal(poll.Options)
	votesJSON, _ := json.Marshal(poll.Votes)

	query := `INSERT INTO polls (id, chat_id, message_mid, question, options, votes, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (id) DO UPDATE SET
	message_mid = EXCLUDED.message_mid;`

	_, err := s.db.ExecContext(ctx, query, poll.ID, poll.ChatID, poll.MessageMID, poll.Question, string(optsJSON), string(votesJSON), poll.CreatedAt)
	return err
}

func (s *SQLStore) GetPoll(id string) (Poll, bool) {
	return s.GetPollContext(context.Background(), id)
}

func (s *SQLStore) GetPollContext(ctx context.Context, id string) (Poll, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, chat_id, message_mid, question, options, votes, created_at FROM polls WHERE id = $1;`
	row := s.db.QueryRowContext(ctx, query, id)

	var p Poll
	var optsStr, votesStr string
	if err := row.Scan(&p.ID, &p.ChatID, &p.MessageMID, &p.Question, &optsStr, &votesStr, &p.CreatedAt); err != nil {
		return Poll{}, false
	}
	_ = json.Unmarshal([]byte(optsStr), &p.Options)
	_ = json.Unmarshal([]byte(votesStr), &p.Votes)
	return p, true
}

func (s *SQLStore) VotePoll(pollID string, userID int64, optionIdx int) ([]int, []int, error) {
	return s.VotePollContext(context.Background(), pollID, userID, optionIdx)
}

func (s *SQLStore) VotePollContext(ctx context.Context, pollID string, userID int64, optionIdx int) ([]int, []int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()

	query := `SELECT id, chat_id, message_mid, question, options, votes, created_at FROM polls WHERE id = $1;`
	if s.dialect == "postgres" {
		query = `SELECT id, chat_id, message_mid, question, options, votes, created_at FROM polls WHERE id = $1 FOR UPDATE;`
	}
	row := tx.QueryRowContext(ctx, query, pollID)

	var p Poll
	var optsStr, votesStr string
	if err := row.Scan(&p.ID, &p.ChatID, &p.MessageMID, &p.Question, &optsStr, &votesStr, &p.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, fmt.Errorf("poll %s not found", pollID)
		}
		return nil, nil, err
	}
	_ = json.Unmarshal([]byte(optsStr), &p.Options)
	_ = json.Unmarshal([]byte(votesStr), &p.Votes)

	if optionIdx < 0 || optionIdx >= len(p.Options) {
		return nil, nil, fmt.Errorf("invalid option index %d", optionIdx)
	}

	if p.Votes == nil {
		p.Votes = make(map[int64]int)
	}
	p.Votes[userID] = optionIdx

	vJSON, _ := json.Marshal(p.Votes)
	updateQuery := `UPDATE polls SET votes = $1 WHERE id = $2;`
	if _, err := tx.ExecContext(ctx, updateQuery, string(vJSON), pollID); err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}

	counts := make([]int, len(p.Options))
	for _, opt := range p.Votes {
		if opt >= 0 && opt < len(counts) {
			counts[opt]++
		}
	}

	total := len(p.Votes)
	percentages := make([]int, len(p.Options))
	if total > 0 {
		for i, c := range counts {
			percentages[i] = int(float64(c) / float64(total) * 100)
		}
	}

	return counts, percentages, nil
}

// --- Giveaway Operations ---

func (s *SQLStore) CreateGiveaway(gw Giveaway) error {
	return s.CreateGiveawayContext(context.Background(), gw)
}

func (s *SQLStore) CreateGiveawayContext(ctx context.Context, gw Giveaway) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if gw.CreatedAt.IsZero() {
		gw.CreatedAt = time.Now()
	}
	if gw.Participants == nil {
		gw.Participants = make(map[int64]string)
	}
	if gw.Winners == nil {
		gw.Winners = make([]int64, 0)
	}
	gw.Active = true

	pJSON, _ := json.Marshal(gw.Participants)
	wJSON, _ := json.Marshal(gw.Winners)

	query := `INSERT INTO giveaways (id, chat_id, title, promo_code, discount_percent, participants, winners, active, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);`

	_, err := s.db.ExecContext(ctx, query, gw.ID, gw.ChatID, gw.Title, gw.PromoCode, gw.DiscountPercent, string(pJSON), string(wJSON), gw.Active, gw.CreatedAt)
	return err
}

func (s *SQLStore) GetGiveaway(id string) (Giveaway, bool) {
	return s.GetGiveawayContext(context.Background(), id)
}

func (s *SQLStore) GetGiveawayContext(ctx context.Context, id string) (Giveaway, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, chat_id, title, promo_code, discount_percent, participants, winners, active, created_at FROM giveaways WHERE id = $1;`
	row := s.db.QueryRowContext(ctx, query, id)

	var gw Giveaway
	var pStr, wStr string
	if err := row.Scan(&gw.ID, &gw.ChatID, &gw.Title, &gw.PromoCode, &gw.DiscountPercent, &pStr, &wStr, &gw.Active, &gw.CreatedAt); err != nil {
		return Giveaway{}, false
	}
	_ = json.Unmarshal([]byte(pStr), &gw.Participants)
	_ = json.Unmarshal([]byte(wStr), &gw.Winners)
	return gw, true
}

func (s *SQLStore) AddGiveawayParticipant(id string, userID int64, userName string) error {
	return s.AddGiveawayParticipantContext(context.Background(), id, userID, userName)
}

func (s *SQLStore) AddGiveawayParticipantContext(ctx context.Context, id string, userID int64, userName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	query := `SELECT participants, active FROM giveaways WHERE id = $1;`
	if s.dialect == "postgres" {
		query = `SELECT participants, active FROM giveaways WHERE id = $1 FOR UPDATE;`
	}
	row := tx.QueryRowContext(ctx, query, id)

	var pStr string
	var active bool
	if err := row.Scan(&pStr, &active); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("giveaway %s not found", id)
		}
		return err
	}
	if !active {
		return fmt.Errorf("giveaway %s is already finished", id)
	}

	participants := make(map[int64]string)
	_ = json.Unmarshal([]byte(pStr), &participants)
	participants[userID] = userName

	pJSON, _ := json.Marshal(participants)
	updateQuery := `UPDATE giveaways SET participants = $1 WHERE id = $2;`
	if _, err := tx.ExecContext(ctx, updateQuery, string(pJSON), id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLStore) DrawGiveawayWinners(id string, count int) ([]int64, error) {
	return s.DrawGiveawayWinnersContext(context.Background(), id, count)
}

func (s *SQLStore) DrawGiveawayWinnersContext(ctx context.Context, id string, count int) ([]int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	query := `SELECT id, chat_id, title, promo_code, discount_percent, participants, winners, active, created_at FROM giveaways WHERE id = $1;`
	if s.dialect == "postgres" {
		query = `SELECT id, chat_id, title, promo_code, discount_percent, participants, winners, active, created_at FROM giveaways WHERE id = $1 FOR UPDATE;`
	}
	row := tx.QueryRowContext(ctx, query, id)

	var gw Giveaway
	var pStr, wStr string
	if err := row.Scan(&gw.ID, &gw.ChatID, &gw.Title, &gw.PromoCode, &gw.DiscountPercent, &pStr, &wStr, &gw.Active, &gw.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("giveaway %s not found", id)
		}
		return nil, err
	}
	if !gw.Active {
		return nil, fmt.Errorf("giveaway %s is already finished", id)
	}
	_ = json.Unmarshal([]byte(pStr), &gw.Participants)

	if len(gw.Participants) == 0 {
		return nil, errors.New("no participants in giveaway")
	}

	uids := make([]int64, 0, len(gw.Participants))
	for uid := range gw.Participants {
		uids = append(uids, uid)
	}

	if err := cryptoShuffle(uids); err != nil {
		return nil, fmt.Errorf("failed to shuffle giveaway participants: %w", err)
	}

	if count > len(uids) {
		count = len(uids)
	}
	winners := uids[:count]

	wJSON, _ := json.Marshal(winners)
	updateQuery := `UPDATE giveaways SET winners = $1, active = false WHERE id = $2;`
	if _, err := tx.ExecContext(ctx, updateQuery, string(wJSON), id); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return winners, nil
}

// --- Media Artifact Operations ---

func (s *SQLStore) SaveMediaArtifact(artifact MediaArtifact) (*MediaArtifact, error) {
	return s.SaveMediaArtifactContext(context.Background(), artifact)
}

func (s *SQLStore) SaveMediaArtifactContext(ctx context.Context, artifact MediaArtifact) (*MediaArtifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if artifact.CreatedAt.IsZero() {
		artifact.CreatedAt = time.Now()
	}
	if artifact.Metadata == nil {
		artifact.Metadata = make(map[string]any)
	}

	metaJSON, err := json.Marshal(artifact.Metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}

	if s.dialect == "postgres" {
		query := `INSERT INTO media_history (user_id, artifact_type, title, prompt, content_text, media_url, metadata, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id;`
		err = s.db.QueryRowContext(ctx, query, artifact.UserID, artifact.ArtifactType, artifact.Title, artifact.Prompt, artifact.ContentText, artifact.MediaURL, string(metaJSON), artifact.CreatedAt).Scan(&artifact.ID)
		if err != nil {
			return nil, err
		}
	} else {
		query := `INSERT INTO media_history (user_id, artifact_type, title, prompt, content_text, media_url, metadata, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);`
		res, err := s.db.ExecContext(ctx, query, artifact.UserID, artifact.ArtifactType, artifact.Title, artifact.Prompt, artifact.ContentText, artifact.MediaURL, string(metaJSON), artifact.CreatedAt)
		if err != nil {
			return nil, err
		}
		artifact.ID, err = res.LastInsertId()
		if err != nil {
			return nil, err
		}
	}

	return &artifact, nil
}

func (s *SQLStore) ListMediaArtifacts(userID int64, artifactType string, limit int) ([]MediaArtifact, error) {
	return s.ListMediaArtifactsContext(context.Background(), userID, artifactType, limit)
}

func (s *SQLStore) ListMediaArtifactsContext(ctx context.Context, userID int64, artifactType string, limit int) ([]MediaArtifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var rows *sql.Rows
	var err error

	if artifactType != "" && artifactType != "all" {
		query := `SELECT id, user_id, artifact_type, title, prompt, content_text, media_url, metadata, created_at
FROM media_history WHERE user_id = $1 AND artifact_type = $2 ORDER BY created_at DESC LIMIT $3;`
		rows, err = s.db.QueryContext(ctx, query, userID, artifactType, limit)
	} else {
		query := `SELECT id, user_id, artifact_type, title, prompt, content_text, media_url, metadata, created_at
FROM media_history WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2;`
		rows, err = s.db.QueryContext(ctx, query, userID, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []MediaArtifact
	for rows.Next() {
		var a MediaArtifact
		var metaStr string
		if err := rows.Scan(&a.ID, &a.UserID, &a.ArtifactType, &a.Title, &a.Prompt, &a.ContentText, &a.MediaURL, &metaStr, &a.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(metaStr), &a.Metadata)
		result = append(result, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// --- Bot Hub Operations ---

// GetUser retrieves a User by internal primary key ID.
func (s *SQLStore) GetUser(ctx context.Context, id int64) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, active_mode_id, role, timezone, created_at, updated_at FROM users WHERE id = $1;`
	row := s.db.QueryRowContext(ctx, query, id)

	var u User
	if err := row.Scan(&u.ID, &u.ActiveModeID, &u.Role, &u.Timezone, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user %d not found: %w", id, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("failed to get user %d: %w", id, err)
	}
	return &u, nil
}

// SetUserActiveMode updates the active mode for a user.
func (s *SQLStore) SetUserActiveMode(ctx context.Context, userID int64, modeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	query := `UPDATE users SET active_mode_id = $1, updated_at = $2 WHERE id = $3;`
	res, err := s.db.ExecContext(ctx, query, modeID, now, userID)
	if err != nil {
		return fmt.Errorf("failed to set user active mode: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user %d not found: %w", userID, sql.ErrNoRows)
	}
	return nil
}

// IsAdminPlatformUser returns true if the platform user ID is listed in ADMIN_USER_IDS.
func IsAdminPlatformUser(platformUserID string) bool {
	if platformUserID == "" {
		return false
	}
	adminEnv := os.Getenv("ADMIN_USER_IDS")
	if adminEnv != "" {
		for _, id := range strings.Split(adminEnv, ",") {
			if strings.TrimSpace(id) == platformUserID {
				return true
			}
		}
	}
	return false
}

// ResolveUserIdentity finds or creates a user and identity for a platform and platformUserID.
func (s *SQLStore) ResolveUserIdentity(ctx context.Context, platform, platformUserID, username, firstName, phone string) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	isAdmin := IsAdminPlatformUser(platformUserID)

	// 1. Check if identity exists
	findQuery := `SELECT id, user_id FROM user_identities WHERE platform = $1 AND platform_user_id = $2;`
	var identityID, userID int64
	err := s.db.QueryRowContext(ctx, findQuery, platform, platformUserID).Scan(&identityID, &userID)
	if err == nil {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer func() { _ = tx.Rollback() }()

		// Update user identity metadata if changed, preserving phone if new one is empty
		updateQuery := `UPDATE user_identities SET username = COALESCE(NULLIF($1, ''), username), first_name = COALESCE(NULLIF($2, ''), first_name), phone = COALESCE(NULLIF($3, ''), phone) WHERE id = $4;`
		if _, err := tx.ExecContext(ctx, updateQuery, username, firstName, phone, identityID); err != nil {
			return nil, fmt.Errorf("failed to update user identity: %w", err)
		}

		var u User
		getUserQuery := `SELECT id, active_mode_id, role, timezone, created_at, updated_at FROM users WHERE id = $1;`
		if err := tx.QueryRowContext(ctx, getUserQuery, userID).Scan(&u.ID, &u.ActiveModeID, &u.Role, &u.Timezone, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to fetch existing user %d: %w", userID, err)
		}
		if isAdmin && u.Role != "admin" && u.Role != "superadmin" {
			if _, err := tx.ExecContext(ctx, `UPDATE users SET role = 'admin', updated_at = $1 WHERE id = $2;`, time.Now(), u.ID); err != nil {
				return nil, fmt.Errorf("failed to promote user %d to admin: %w", u.ID, err)
			}
			u.Role = "admin"
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit user identity update: %w", err)
		}
		return &u, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to lookup user identity: %w", err)
	}

	// 2. Not found: create new user and identity atomically within a transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()
	var user User
	user.ActiveModeID = "chat"
	user.Role = "user"
	if isAdmin {
		user.Role = "admin"
	}
	user.Timezone = "UTC+3"
	user.CreatedAt = now
	user.UpdatedAt = now

	if s.dialect == "postgres" {
		insertUserQuery := `INSERT INTO users (active_mode_id, role, timezone, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5) RETURNING id;`
		if err := tx.QueryRowContext(ctx, insertUserQuery, user.ActiveModeID, user.Role, user.Timezone, user.CreatedAt, user.UpdatedAt).Scan(&user.ID); err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	} else {
		insertUserQuery := `INSERT INTO users (active_mode_id, role, timezone, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5);`
		res, err := tx.ExecContext(ctx, insertUserQuery, user.ActiveModeID, user.Role, user.Timezone, user.CreatedAt, user.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to create user in sqlite: %w", err)
		}
		user.ID, err = res.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("failed to get last insert id: %w", err)
		}
	}

	// 3. Create identity linked to user
	insertIdentQuery := `INSERT INTO user_identities (user_id, platform, platform_user_id, username, first_name, phone, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);`
	if _, err := tx.ExecContext(ctx, insertIdentQuery, user.ID, platform, platformUserID, username, firstName, phone, now); err != nil {
		return nil, fmt.Errorf("failed to create user identity: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit user and identity transaction: %w", err)
	}

	return &user, nil
}

// GetUserModeState fetches the composite FSM state for a user and mode.
func (s *SQLStore) GetUserModeState(ctx context.Context, userID int64, modeID string) (*UserModeState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT user_id, mode_id, current_step, payload, updated_at FROM user_mode_states WHERE user_id = $1 AND mode_id = $2;`
	row := s.db.QueryRowContext(ctx, query, userID, modeID)

	var st UserModeState
	var payloadStr string
	if err := row.Scan(&st.UserID, &st.ModeID, &st.CurrentStep, &payloadStr, &st.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("mode state not found for user %d mode %s: %w", userID, modeID, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("failed to get user mode state: %w", err)
	}
	st.Payload = json.RawMessage(payloadStr)
	return &st, nil
}

// SaveUserModeState creates or updates composite FSM state for a user and mode.
func (s *SQLStore) SaveUserModeState(ctx context.Context, state *UserModeState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if state == nil {
		return errors.New("state cannot be nil")
	}
	if len(state.Payload) == 0 {
		state.Payload = json.RawMessage("{}")
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now()
	}

	query := `INSERT INTO user_mode_states (user_id, mode_id, current_step, payload, updated_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, mode_id) DO UPDATE SET
	current_step = EXCLUDED.current_step,
	payload = EXCLUDED.payload,
	updated_at = EXCLUDED.updated_at;`

	_, err := s.db.ExecContext(ctx, query, state.UserID, state.ModeID, state.CurrentStep, string(state.Payload), state.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to save user mode state: %w", err)
	}
	return nil
}

// GetModeConfigs retrieves all mode configurations and maintenance switch flags.
func (s *SQLStore) GetModeConfigs(ctx context.Context) ([]ModeConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT mode_id, title, is_active, maintenance_message, daily_quota, updated_at FROM mode_configs ORDER BY mode_id ASC;`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get mode configs: %w", err)
	}
	defer rows.Close()

	var configs []ModeConfig
	for rows.Next() {
		var cfg ModeConfig
		if err := rows.Scan(&cfg.ModeID, &cfg.Title, &cfg.IsActive, &cfg.MaintenanceMessage, &cfg.DailyQuota, &cfg.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan mode config: %w", err)
		}
		configs = append(configs, cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error in mode configs: %w", err)
	}
	return configs, nil
}

// SetModeActive updates the active status and optional maintenance message for a mode.
func (s *SQLStore) SetModeActive(ctx context.Context, modeID string, isActive bool, maintMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction for SetModeActive: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now()
	var res sql.Result
	if maintMsg != "" {
		query := `UPDATE mode_configs SET is_active = $1, maintenance_message = $2, updated_at = $3 WHERE mode_id = $4;`
		res, err = tx.ExecContext(ctx, query, isActive, maintMsg, now, modeID)
	} else {
		query := `UPDATE mode_configs SET is_active = $1, updated_at = $2 WHERE mode_id = $3;`
		res, err = tx.ExecContext(ctx, query, isActive, now, modeID)
	}
	if err != nil {
		return fmt.Errorf("failed to set mode active for %s: %w", modeID, err)
	}

	if res != nil {
		rows, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to check rows affected for %s: %w", modeID, err)
		}
		if rows == 0 {
			// Mode didn't exist in mode_configs, insert new row
			insertQuery := `INSERT INTO mode_configs (mode_id, title, is_active, maintenance_message, daily_quota, updated_at) VALUES ($1, $2, $3, $4, 100, $5);`
			if _, err := tx.ExecContext(ctx, insertQuery, modeID, modeID, isActive, maintMsg, now); err != nil {
				return fmt.Errorf("failed to insert mode config for %s: %w", modeID, err)
			}
		}
	}
	return tx.Commit()
}

// SaveLead creates a new lead and returns its generated ID.
func (s *SQLStore) SaveLead(ctx context.Context, lead *Lead) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if lead == nil {
		return 0, errors.New("lead cannot be nil")
	}
	if lead.Status == "" {
		lead.Status = "new"
	}
	if len(lead.TestedModes) == 0 {
		lead.TestedModes = json.RawMessage("[]")
	}
	if lead.CreatedAt.IsZero() {
		lead.CreatedAt = time.Now()
	}

	if s.dialect == "postgres" {
		query := `INSERT INTO leads (user_id, platform, source_mode, tested_modes, contact, comment, status, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id;`
		err := s.db.QueryRowContext(ctx, query, lead.UserID, lead.Platform, lead.SourceMode, string(lead.TestedModes), lead.Contact, lead.Comment, lead.Status, lead.CreatedAt).Scan(&lead.ID)
		if err != nil {
			return 0, fmt.Errorf("failed to save lead in postgres: %w", err)
		}
	} else {
		query := `INSERT INTO leads (user_id, platform, source_mode, tested_modes, contact, comment, status, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);`
		res, err := s.db.ExecContext(ctx, query, lead.UserID, lead.Platform, lead.SourceMode, string(lead.TestedModes), lead.Contact, lead.Comment, lead.Status, lead.CreatedAt)
		if err != nil {
			return 0, fmt.Errorf("failed to save lead in sqlite: %w", err)
		}
		lead.ID, err = res.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("failed to get lead last insert id: %w", err)
		}
	}
	return lead.ID, nil
}

// GetLeads retrieves leads optionally filtered by status, ordered by created_at descending.
func (s *SQLStore) GetLeads(ctx context.Context, status string) ([]Lead, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var rows *sql.Rows
	var err error
	if status != "" {
		query := `SELECT id, user_id, platform, source_mode, tested_modes, contact, comment, status, created_at FROM leads WHERE status = $1 ORDER BY created_at DESC;`
		rows, err = s.db.QueryContext(ctx, query, status)
	} else {
		query := `SELECT id, user_id, platform, source_mode, tested_modes, contact, comment, status, created_at FROM leads ORDER BY created_at DESC;`
		rows, err = s.db.QueryContext(ctx, query)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query leads: %w", err)
	}
	defer rows.Close()

	var leads []Lead
	for rows.Next() {
		var l Lead
		var modesStr string
		if err := rows.Scan(&l.ID, &l.UserID, &l.Platform, &l.SourceMode, &modesStr, &l.Contact, &l.Comment, &l.Status, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan lead: %w", err)
		}
		l.TestedModes = json.RawMessage(modesStr)
		leads = append(leads, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error in leads: %w", err)
	}
	return leads, nil
}

// GetMascotPhrases retrieves active mascot phrases ordered by priority ascending.
func (s *SQLStore) GetMascotPhrases(ctx context.Context) ([]MascotPhrase, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var query string
	if s.dialect == "postgres" {
		query = `SELECT id, priority, trigger_type, text, is_active FROM mascot_phrases WHERE is_active = TRUE ORDER BY priority ASC, id ASC;`
	} else {
		query = `SELECT id, priority, trigger_type, text, is_active FROM mascot_phrases WHERE is_active = 1 ORDER BY priority ASC, id ASC;`
	}
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query mascot phrases: %w", err)
	}
	defer rows.Close()

	var phrases []MascotPhrase
	for rows.Next() {
		var p MascotPhrase
		if err := rows.Scan(&p.ID, &p.Priority, &p.TriggerType, &p.Text, &p.IsActive); err != nil {
			return nil, fmt.Errorf("failed to scan mascot phrase: %w", err)
		}
		phrases = append(phrases, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error in mascot phrases: %w", err)
	}
	return phrases, nil
}

// UpdateLeadStatus updates the status of an existing lead.
func (s *SQLStore) UpdateLeadStatus(ctx context.Context, leadID int64, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `UPDATE leads SET status = $1 WHERE id = $2;`
	res, err := s.db.ExecContext(ctx, query, status, leadID)
	if err != nil {
		return fmt.Errorf("failed to update lead status: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("lead %d not found: %w", leadID, sql.ErrNoRows)
	}
	return nil
}

// CreateSupportTicket creates a new support ticket and returns its generated ID.
func (s *SQLStore) CreateSupportTicket(ctx context.Context, ticket *SupportTicket) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ticket == nil {
		return 0, errors.New("ticket cannot be nil")
	}
	if ticket.Status == "" {
		ticket.Status = "open"
	}
	now := time.Now()
	if ticket.CreatedAt.IsZero() {
		ticket.CreatedAt = now
	}
	if ticket.UpdatedAt.IsZero() {
		ticket.UpdatedAt = now
	}

	if s.dialect == "postgres" {
		query := `INSERT INTO support_tickets (user_id, platform, mode_scope, subject, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id;`
		err := s.db.QueryRowContext(ctx, query, ticket.UserID, ticket.Platform, ticket.ModeScope, ticket.Subject, ticket.Status, ticket.CreatedAt, ticket.UpdatedAt).Scan(&ticket.ID)
		if err != nil {
			return 0, fmt.Errorf("failed to create support ticket in postgres: %w", err)
		}
	} else {
		query := `INSERT INTO support_tickets (user_id, platform, mode_scope, subject, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);`
		res, err := s.db.ExecContext(ctx, query, ticket.UserID, ticket.Platform, ticket.ModeScope, ticket.Subject, ticket.Status, ticket.CreatedAt, ticket.UpdatedAt)
		if err != nil {
			return 0, fmt.Errorf("failed to create support ticket in sqlite: %w", err)
		}
		ticket.ID, err = res.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("failed to get support ticket last insert id: %w", err)
		}
	}
	return ticket.ID, nil
}

// CreateSupportTicketWithMessage atomically creates a support ticket and its initial message in a transaction.
func (s *SQLStore) CreateSupportTicketWithMessage(ctx context.Context, ticket *SupportTicket, msg *SupportMessage) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ticket == nil {
		return 0, errors.New("ticket cannot be nil")
	}
	if ticket.Status == "" {
		ticket.Status = "open"
	}
	now := time.Now()
	if ticket.CreatedAt.IsZero() {
		ticket.CreatedAt = now
	}
	if ticket.UpdatedAt.IsZero() {
		ticket.UpdatedAt = now
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if s.dialect == "postgres" {
		query := `INSERT INTO support_tickets (user_id, platform, mode_scope, subject, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id;`
		if err := tx.QueryRowContext(ctx, query, ticket.UserID, ticket.Platform, ticket.ModeScope, ticket.Subject, ticket.Status, ticket.CreatedAt, ticket.UpdatedAt).Scan(&ticket.ID); err != nil {
			return 0, fmt.Errorf("failed to create support ticket in postgres: %w", err)
		}
	} else {
		query := `INSERT INTO support_tickets (user_id, platform, mode_scope, subject, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);`
		res, err := tx.ExecContext(ctx, query, ticket.UserID, ticket.Platform, ticket.ModeScope, ticket.Subject, ticket.Status, ticket.CreatedAt, ticket.UpdatedAt)
		if err != nil {
			return 0, fmt.Errorf("failed to create support ticket in sqlite: %w", err)
		}
		ticket.ID, err = res.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("failed to get support ticket last insert id: %w", err)
		}
	}

	if msg != nil {
		msg.TicketID = ticket.ID
		if msg.CreatedAt.IsZero() {
			msg.CreatedAt = now
		}
		if s.dialect == "postgres" {
			msgQuery := `INSERT INTO support_messages (ticket_id, sender_type, message_text, created_at)
VALUES ($1, $2, $3, $4) RETURNING id;`
			if err := tx.QueryRowContext(ctx, msgQuery, msg.TicketID, msg.SenderType, msg.MessageText, msg.CreatedAt).Scan(&msg.ID); err != nil {
				return 0, fmt.Errorf("failed to save initial support message in postgres: %w", err)
			}
		} else {
			msgQuery := `INSERT INTO support_messages (ticket_id, sender_type, message_text, created_at)
VALUES ($1, $2, $3, $4);`
			res, err := tx.ExecContext(ctx, msgQuery, msg.TicketID, msg.SenderType, msg.MessageText, msg.CreatedAt)
			if err != nil {
				return 0, fmt.Errorf("failed to save initial support message in sqlite: %w", err)
			}
			msg.ID, err = res.LastInsertId()
			if err != nil {
				return 0, fmt.Errorf("failed to get initial support message last insert id: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit support ticket transaction: %w", err)
	}
	return ticket.ID, nil
}

// GetSupportTickets retrieves support tickets optionally filtered by status, ordered by created_at descending.
func (s *SQLStore) GetSupportTickets(ctx context.Context, status string) ([]SupportTicket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var rows *sql.Rows
	var err error
	if status != "" {
		query := `SELECT t.id, t.user_id, t.platform, t.mode_scope, t.subject, t.status, t.created_at, t.updated_at,
		                 COALESCE((SELECT m.message_text FROM support_messages m WHERE m.ticket_id = t.id AND m.sender_type = 'user' ORDER BY m.id ASC LIMIT 1), '') as message
		          FROM support_tickets t WHERE t.status = $1 ORDER BY t.created_at DESC;`
		rows, err = s.db.QueryContext(ctx, query, status)
	} else {
		query := `SELECT t.id, t.user_id, t.platform, t.mode_scope, t.subject, t.status, t.created_at, t.updated_at,
		                 COALESCE((SELECT m.message_text FROM support_messages m WHERE m.ticket_id = t.id AND m.sender_type = 'user' ORDER BY m.id ASC LIMIT 1), '') as message
		          FROM support_tickets t ORDER BY t.created_at DESC;`
		rows, err = s.db.QueryContext(ctx, query)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query support tickets: %w", err)
	}
	defer rows.Close()

	var tickets []SupportTicket
	for rows.Next() {
		var t SupportTicket
		if err := rows.Scan(&t.ID, &t.UserID, &t.Platform, &t.ModeScope, &t.Subject, &t.Status, &t.CreatedAt, &t.UpdatedAt, &t.Message); err != nil {
			return nil, fmt.Errorf("failed to scan support ticket: %w", err)
		}
		tickets = append(tickets, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error in support tickets: %w", err)
	}
	return tickets, nil
}

// GetSupportTicket retrieves a single support ticket by ID.
func (s *SQLStore) GetSupportTicket(ctx context.Context, ticketID int64) (*SupportTicket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT t.id, t.user_id, t.platform, t.mode_scope, t.subject, t.status, t.created_at, t.updated_at,
	                 COALESCE((SELECT m.message_text FROM support_messages m WHERE m.ticket_id = t.id AND m.sender_type = 'user' ORDER BY m.id ASC LIMIT 1), '') as message
	          FROM support_tickets t WHERE t.id = $1;`
	row := s.db.QueryRowContext(ctx, query, ticketID)

	var t SupportTicket
	if err := row.Scan(&t.ID, &t.UserID, &t.Platform, &t.ModeScope, &t.Subject, &t.Status, &t.CreatedAt, &t.UpdatedAt, &t.Message); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("support ticket %d not found: %w", ticketID, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("failed to get support ticket %d: %w", ticketID, err)
	}
	return &t, nil
}

// UpdateSupportTicketStatus updates the status and updated_at timestamp of a support ticket.
func (s *SQLStore) UpdateSupportTicketStatus(ctx context.Context, ticketID int64, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	query := `UPDATE support_tickets SET status = $1, updated_at = $2 WHERE id = $3;`
	res, err := s.db.ExecContext(ctx, query, status, now, ticketID)
	if err != nil {
		return fmt.Errorf("failed to update support ticket status: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("support ticket %d not found: %w", ticketID, sql.ErrNoRows)
	}
	return nil
}

// SaveSupportMessage creates a new support message linked to a ticket.
func (s *SQLStore) SaveSupportMessage(ctx context.Context, msg *SupportMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if msg == nil {
		return errors.New("support message cannot be nil")
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}

	if s.dialect == "postgres" {
		query := `INSERT INTO support_messages (ticket_id, sender_type, message_text, created_at)
VALUES ($1, $2, $3, $4) RETURNING id;`
		err := s.db.QueryRowContext(ctx, query, msg.TicketID, msg.SenderType, msg.MessageText, msg.CreatedAt).Scan(&msg.ID)
		if err != nil {
			return fmt.Errorf("failed to save support message in postgres: %w", err)
		}
	} else {
		query := `INSERT INTO support_messages (ticket_id, sender_type, message_text, created_at)
VALUES ($1, $2, $3, $4);`
		res, err := s.db.ExecContext(ctx, query, msg.TicketID, msg.SenderType, msg.MessageText, msg.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to save support message in sqlite: %w", err)
		}
		msg.ID, err = res.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get support message last insert id: %w", err)
		}
	}
	return nil
}

// GetSupportMessages retrieves all messages for a ticket ordered by created_at ascending.
func (s *SQLStore) GetSupportMessages(ctx context.Context, ticketID int64) ([]SupportMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, ticket_id, sender_type, message_text, created_at FROM support_messages WHERE ticket_id = $1 ORDER BY created_at ASC;`
	rows, err := s.db.QueryContext(ctx, query, ticketID)
	if err != nil {
		return nil, fmt.Errorf("failed to query support messages: %w", err)
	}
	defer rows.Close()

	var msgs []SupportMessage
	for rows.Next() {
		var m SupportMessage
		if err := rows.Scan(&m.ID, &m.TicketID, &m.SenderType, &m.MessageText, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan support message: %w", err)
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error in support messages: %w", err)
	}
	return msgs, nil
}

// SaveAdminAuditLog records an admin action in the audit log table.
func (s *SQLStore) SaveAdminAuditLog(ctx context.Context, log *AdminAuditLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if log == nil {
		return errors.New("audit log cannot be nil")
	}
	if len(log.PayloadDiff) == 0 {
		log.PayloadDiff = json.RawMessage("{}")
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}

	if s.dialect == "postgres" {
		query := `INSERT INTO admin_audit_logs (admin_id, action, target_mode, payload_diff, ip, created_at)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING id;`
		err := s.db.QueryRowContext(ctx, query, log.AdminID, log.Action, log.TargetMode, string(log.PayloadDiff), log.IP, log.CreatedAt).Scan(&log.ID)
		if err != nil {
			return fmt.Errorf("failed to save admin audit log in postgres: %w", err)
		}
	} else {
		query := `INSERT INTO admin_audit_logs (admin_id, action, target_mode, payload_diff, ip, created_at)
VALUES ($1, $2, $3, $4, $5, $6);`
		res, err := s.db.ExecContext(ctx, query, log.AdminID, log.Action, log.TargetMode, string(log.PayloadDiff), log.IP, log.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to save admin audit log in sqlite: %w", err)
		}
		log.ID, err = res.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get admin audit log last insert id: %w", err)
		}
	}
	return nil
}

// GetAdminAuditLogs retrieves recent audit logs ordered by created_at descending.
func (s *SQLStore) GetAdminAuditLogs(ctx context.Context, limit int) ([]AdminAuditLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	query := `SELECT id, admin_id, action, target_mode, payload_diff, ip, created_at FROM admin_audit_logs ORDER BY created_at DESC, id DESC LIMIT $1;`
	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query admin audit logs: %w", err)
	}
	defer rows.Close()

	var logs []AdminAuditLog
	for rows.Next() {
		var l AdminAuditLog
		var targetMode sql.NullString
		var diffStr sql.NullString
		var ip sql.NullString
		if err := rows.Scan(&l.ID, &l.AdminID, &l.Action, &targetMode, &diffStr, &ip, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan admin audit log: %w", err)
		}
		if targetMode.Valid {
			l.TargetMode = targetMode.String
		}
		if ip.Valid {
			l.IP = ip.String
		}
		if diffStr.Valid && diffStr.String != "" {
			l.PayloadDiff = json.RawMessage(diffStr.String)
		} else {
			l.PayloadDiff = json.RawMessage("{}")
		}
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error in admin audit logs: %w", err)
	}
	return logs, nil
}

// GetUserIdentity retrieves the user identity for a given user ID and platform.
func (s *SQLStore) GetUserIdentity(ctx context.Context, userID int64, platform string) (*UserIdentity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var query string
	var row *sql.Row
	if platform != "" {
		query = `SELECT id, user_id, platform, platform_user_id, username, first_name, phone, created_at FROM user_identities WHERE user_id = $1 AND platform = $2 LIMIT 1;`
		row = s.db.QueryRowContext(ctx, query, userID, platform)
	} else {
		query = `SELECT id, user_id, platform, platform_user_id, username, first_name, phone, created_at FROM user_identities WHERE user_id = $1 LIMIT 1;`
		row = s.db.QueryRowContext(ctx, query, userID)
	}

	var ident UserIdentity
	var username, firstName, phone sql.NullString
	if err := row.Scan(&ident.ID, &ident.UserID, &ident.Platform, &ident.PlatformUserID, &username, &firstName, &phone, &ident.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user identity not found: %w", sql.ErrNoRows)
		}
		return nil, fmt.Errorf("failed to query user identity: %w", err)
	}
	if username.Valid {
		ident.Username = username.String
	}
	if firstName.Valid {
		ident.FirstName = firstName.String
	}
	if phone.Valid {
		ident.Phone = phone.String
	}
	return &ident, nil
}

// CreateReminder inserts a new reminder into the database.
func (s *SQLStore) CreateReminder(ctx context.Context, rem *Reminder) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if rem == nil {
		return 0, errors.New("reminder cannot be nil")
	}
	if rem.Status == "" {
		rem.Status = "pending"
	}
	if rem.RecurrenceRule == "" {
		rem.RecurrenceRule = "none"
	}
	if rem.SourceType == "" {
		rem.SourceType = "user"
	}
	if rem.ABVariant == "" {
		rem.ABVariant = "A"
	}
	now := time.Now().UTC()
	if rem.CreatedAt.IsZero() {
		rem.CreatedAt = now
	}
	if rem.UpdatedAt.IsZero() {
		rem.UpdatedAt = now
	}

	var templateID sql.NullString
	if rem.TemplateID != "" {
		templateID = sql.NullString{String: rem.TemplateID, Valid: true}
	}

	if s.dialect == "postgres" {
		query := `INSERT INTO reminders (user_id, title, description, remind_at, recurrence_rule, status, source_type, source_ref_id, template_id, ab_variant, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING id;`
		err := s.db.QueryRowContext(ctx, query,
			rem.UserID, rem.Title, rem.Description, rem.RemindAt.UTC(), rem.RecurrenceRule, rem.Status,
			rem.SourceType, rem.SourceRefID, templateID, rem.ABVariant, rem.CreatedAt, rem.UpdatedAt,
		).Scan(&rem.ID)
		if err != nil {
			return 0, fmt.Errorf("failed to create reminder in postgres: %w", err)
		}
	} else {
		query := `INSERT INTO reminders (user_id, title, description, remind_at, recurrence_rule, status, source_type, source_ref_id, template_id, ab_variant, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`
		res, err := s.db.ExecContext(ctx, query,
			rem.UserID, rem.Title, rem.Description, rem.RemindAt.UTC(), rem.RecurrenceRule, rem.Status,
			rem.SourceType, rem.SourceRefID, templateID, rem.ABVariant, rem.CreatedAt, rem.UpdatedAt,
		)
		if err != nil {
			return 0, fmt.Errorf("failed to create reminder in sqlite: %w", err)
		}
		rem.ID, err = res.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("failed to get reminder last insert id: %w", err)
		}
	}
	return rem.ID, nil
}

// GetRemindersByUser returns all reminders for a given user, optionally filtered by status.
func (s *SQLStore) GetRemindersByUser(ctx context.Context, userID int64, statusFilter string) ([]Reminder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var query string
	var args []any

	paramPlaceholder := func(idx int) string {
		if s.dialect == "postgres" {
			return fmt.Sprintf("$%d", idx)
		}
		return "?"
	}

	if statusFilter == "" || statusFilter == "all" {
		query = fmt.Sprintf(`SELECT id, user_id, title, description, remind_at, recurrence_rule, status, source_type, source_ref_id, template_id, ab_variant, last_sent_at, created_at, updated_at
FROM reminders WHERE user_id = %s ORDER BY remind_at ASC;`, paramPlaceholder(1))
		args = append(args, userID)
	} else if statusFilter == "active" {
		query = fmt.Sprintf(`SELECT id, user_id, title, description, remind_at, recurrence_rule, status, source_type, source_ref_id, template_id, ab_variant, last_sent_at, created_at, updated_at
FROM reminders WHERE user_id = %s AND status IN ('pending', 'sent') ORDER BY remind_at ASC;`, paramPlaceholder(1))
		args = append(args, userID)
	} else {
		query = fmt.Sprintf(`SELECT id, user_id, title, description, remind_at, recurrence_rule, status, source_type, source_ref_id, template_id, ab_variant, last_sent_at, created_at, updated_at
FROM reminders WHERE user_id = %s AND status = %s ORDER BY remind_at ASC;`, paramPlaceholder(1), paramPlaceholder(2))
		args = append(args, userID, statusFilter)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query reminders: %w", err)
	}
	defer rows.Close()

	var results []Reminder
	for rows.Next() {
		var rem Reminder
		var templateID sql.NullString
		var lastSentAt sql.NullTime
		if err := rows.Scan(
			&rem.ID, &rem.UserID, &rem.Title, &rem.Description, &rem.RemindAt,
			&rem.RecurrenceRule, &rem.Status, &rem.SourceType, &rem.SourceRefID,
			&templateID, &rem.ABVariant, &lastSentAt, &rem.CreatedAt, &rem.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan reminder row: %w", err)
		}
		if templateID.Valid {
			rem.TemplateID = templateID.String
		}
		if lastSentAt.Valid {
			t := lastSentAt.Time
			rem.LastSentAt = &t
		}
		results = append(results, rem)
	}
	return results, rows.Err()
}

// GetReminderByID fetches a single reminder by ID.
func (s *SQLStore) GetReminderByID(ctx context.Context, id int64) (*Reminder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	param := "?"
	if s.dialect == "postgres" {
		param = "$1"
	}
	query := fmt.Sprintf(`SELECT id, user_id, title, description, remind_at, recurrence_rule, status, source_type, source_ref_id, template_id, ab_variant, last_sent_at, created_at, updated_at
FROM reminders WHERE id = %s;`, param)

	row := s.db.QueryRowContext(ctx, query, id)
	var rem Reminder
	var templateID sql.NullString
	var lastSentAt sql.NullTime
	if err := row.Scan(
		&rem.ID, &rem.UserID, &rem.Title, &rem.Description, &rem.RemindAt,
		&rem.RecurrenceRule, &rem.Status, &rem.SourceType, &rem.SourceRefID,
		&templateID, &rem.ABVariant, &lastSentAt, &rem.CreatedAt, &rem.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("reminder not found: %w", sql.ErrNoRows)
		}
		return nil, fmt.Errorf("failed to scan reminder: %w", err)
	}
	if templateID.Valid {
		rem.TemplateID = templateID.String
	}
	if lastSentAt.Valid {
		t := lastSentAt.Time
		rem.LastSentAt = &t
	}
	return &rem, nil
}

// UpdateReminderStatus updates the status and updated_at of a reminder.
func (s *SQLStore) UpdateReminderStatus(ctx context.Context, id int64, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	var query string
	if s.dialect == "postgres" {
		query = `UPDATE reminders SET status = $1, updated_at = $2, last_sent_at = CASE WHEN $1 = 'sent' THEN $2 ELSE last_sent_at END WHERE id = $3;`
	} else {
		query = `UPDATE reminders SET status = ?, updated_at = ?, last_sent_at = CASE WHEN ? = 'sent' THEN ? ELSE last_sent_at END WHERE id = ?;`
	}

	var err error
	if s.dialect == "postgres" {
		_, err = s.db.ExecContext(ctx, query, status, now, id)
	} else {
		_, err = s.db.ExecContext(ctx, query, status, now, status, now, id)
	}
	if err != nil {
		return fmt.Errorf("failed to update reminder status: %w", err)
	}
	return nil
}

// SnoozeReminderInStore shifts the remind_at time and resets status to 'pending'.
func (s *SQLStore) SnoozeReminderInStore(ctx context.Context, id int64, newRemindAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	var query string
	if s.dialect == "postgres" {
		query = `UPDATE reminders SET remind_at = $1, status = 'pending', updated_at = $2 WHERE id = $3;`
	} else {
		query = `UPDATE reminders SET remind_at = ?, status = 'pending', updated_at = ? WHERE id = ?;`
	}
	_, err := s.db.ExecContext(ctx, query, newRemindAt.UTC(), now, id)
	if err != nil {
		return fmt.Errorf("failed to snooze reminder: %w", err)
	}
	return nil
}

// CancelReminderBySourceRef cancels pending reminders linked to a specific source reference.
func (s *SQLStore) CancelReminderBySourceRef(ctx context.Context, sourceType, refID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	var query string
	if s.dialect == "postgres" {
		query = `UPDATE reminders SET status = 'cancelled', updated_at = $1 WHERE source_type = $2 AND source_ref_id = $3 AND status != 'cancelled';`
	} else {
		query = `UPDATE reminders SET status = 'cancelled', updated_at = ? WHERE source_type = ? AND source_ref_id = ? AND status != 'cancelled';`
	}
	_, err := s.db.ExecContext(ctx, query, now, sourceType, refID)
	if err != nil {
		return fmt.Errorf("failed to cancel reminder by source ref: %w", err)
	}
	return nil
}

// TryUpdateReminderStatus atomically transitions a reminder's status only if its current status is in fromStatuses.
func (s *SQLStore) TryUpdateReminderStatus(ctx context.Context, id int64, newStatus string, fromStatuses ...string) (bool, error) {
	if len(fromStatuses) == 0 {
		return false, errors.New("fromStatuses cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	var args []any
	var query string
	if s.dialect == "postgres" {
		args = append(args, newStatus, now, id)
		placeholders := make([]string, len(fromStatuses))
		for i, st := range fromStatuses {
			placeholders[i] = fmt.Sprintf("$%d", i+4)
			args = append(args, st)
		}
		query = fmt.Sprintf(
			`UPDATE reminders SET status = $1, updated_at = $2, last_sent_at = CASE WHEN $1 = 'sent' THEN $2 ELSE last_sent_at END WHERE id = $3 AND status IN (%s);`,
			strings.Join(placeholders, ","),
		)
	} else {
		args = append(args, newStatus, now, newStatus, now, id)
		placeholders := make([]string, len(fromStatuses))
		for i, st := range fromStatuses {
			placeholders[i] = "?"
			args = append(args, st)
		}
		query = fmt.Sprintf(
			`UPDATE reminders SET status = ?, updated_at = ?, last_sent_at = CASE WHEN ? = 'sent' THEN ? ELSE last_sent_at END WHERE id = ? AND status IN (%s);`,
			strings.Join(placeholders, ","),
		)
	}

	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return false, fmt.Errorf("failed to update reminder status: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to inspect affected rows: %w", err)
	}
	return affected > 0, nil
}

// GetPendingRemindersForUpdate atomically claims a batch of pending reminders due for delivery.
func (s *SQLStore) GetPendingRemindersForUpdate(ctx context.Context, limit int) ([]Reminder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	if limit <= 0 {
		limit = 50
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction for pending reminders: %w", err)
	}
	defer tx.Rollback()

	var rows *sql.Rows
	if s.dialect == "postgres" {
		query := `UPDATE reminders
SET status = 'processing', updated_at = $1
WHERE id IN (
	SELECT id
	FROM reminders
	WHERE status = 'pending' AND remind_at <= $1
	ORDER BY remind_at ASC
	LIMIT $2
	FOR UPDATE SKIP LOCKED
)
RETURNING id, user_id, title, description, remind_at, recurrence_rule, status, source_type, source_ref_id, template_id, ab_variant, last_sent_at, created_at, updated_at;`
		rows, err = tx.QueryContext(ctx, query, now, limit)
	} else {
		query := `SELECT id, user_id, title, description, remind_at, recurrence_rule, status, source_type, source_ref_id, template_id, ab_variant, last_sent_at, created_at, updated_at
FROM reminders
WHERE status = 'pending' AND remind_at <= ?
ORDER BY remind_at ASC
LIMIT ?;`
		rows, err = tx.QueryContext(ctx, query, now, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query pending reminders: %w", err)
	}

	var results []Reminder
	var claimedIDs []any
	for rows.Next() {
		var rem Reminder
		var templateID sql.NullString
		var lastSentAt sql.NullTime
		if err := rows.Scan(
			&rem.ID, &rem.UserID, &rem.Title, &rem.Description, &rem.RemindAt,
			&rem.RecurrenceRule, &rem.Status, &rem.SourceType, &rem.SourceRefID,
			&templateID, &rem.ABVariant, &lastSentAt, &rem.CreatedAt, &rem.UpdatedAt,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("failed to scan pending reminder row: %w", err)
		}
		if templateID.Valid {
			rem.TemplateID = templateID.String
		}
		if lastSentAt.Valid {
			t := lastSentAt.Time
			rem.LastSentAt = &t
		}
		results = append(results, rem)
		claimedIDs = append(claimedIDs, rem.ID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	if s.dialect != "postgres" && len(claimedIDs) > 0 {
		placeholders := make([]string, len(claimedIDs))
		args := make([]any, 0, len(claimedIDs)+1)
		args = append(args, now)
		for i, id := range claimedIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		claimQuery := fmt.Sprintf(
			`UPDATE reminders SET status = 'processing', updated_at = ? WHERE id IN (%s);`,
			strings.Join(placeholders, ","),
		)
		if _, err := tx.ExecContext(ctx, claimQuery, args...); err != nil {
			return nil, fmt.Errorf("failed to claim pending reminders: %w", err)
		}
		for i := range results {
			results[i].Status = "processing"
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit pending reminders claim: %w", err)
	}
	return results, nil
}

// RecordNotificationEvent records an interaction or delivery event for A/B testing analytics.
func (s *SQLStore) RecordNotificationEvent(ctx context.Context, ev *NotificationEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ev == nil {
		return errors.New("notification event cannot be nil")
	}
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now().UTC()
	}
	if ev.Variant == "" {
		ev.Variant = "A"
	}

	var templateID sql.NullString
	if ev.TemplateID != "" {
		templateID = sql.NullString{String: ev.TemplateID, Valid: true}
	}

	var query string
	if s.dialect == "postgres" {
		query = `INSERT INTO notification_events (reminder_id, template_id, user_id, variant, event_type, created_at)
VALUES ($1, $2, $3, $4, $5, $6);`
	} else {
		query = `INSERT INTO notification_events (reminder_id, template_id, user_id, variant, event_type, created_at)
VALUES (?, ?, ?, ?, ?, ?);`
	}

	_, err := s.db.ExecContext(ctx, query, ev.ReminderID, templateID, ev.UserID, ev.Variant, ev.EventType, ev.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert notification event: %w", err)
	}
	return nil
}

// GetNotificationTemplate fetches a notification template by ID.
func (s *SQLStore) GetNotificationTemplate(ctx context.Context, id string) (*NotificationTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	param := "?"
	if s.dialect == "postgres" {
		param = "$1"
	}
	query := fmt.Sprintf(`SELECT id, title, description, is_ab_test_active, variant_a_text, variant_a_buttons, variant_b_text, variant_b_buttons, split_ratio_percent, created_at, updated_at
FROM notification_templates WHERE id = %s;`, param)

	row := s.db.QueryRowContext(ctx, query, id)
	var tmpl NotificationTemplate
	if err := row.Scan(
		&tmpl.ID, &tmpl.Title, &tmpl.Description, &tmpl.IsABTestActive,
		&tmpl.VariantAText, &tmpl.VariantAButtons, &tmpl.VariantBText,
		&tmpl.VariantBButtons, &tmpl.SplitRatioPercent, &tmpl.CreatedAt, &tmpl.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("notification template not found: %w", sql.ErrNoRows)
		}
		return nil, fmt.Errorf("failed to scan notification template: %w", err)
	}
	return &tmpl, nil
}

// GetNotificationABStats aggregates delivery and conversion events grouped by template and variant.
func (s *SQLStore) GetNotificationABStats(ctx context.Context) ([]NotificationABStat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `SELECT COALESCE(template_id, 'custom') AS template_id, variant,
SUM(CASE WHEN event_type = 'sent' THEN 1 ELSE 0 END) AS sent_count,
SUM(CASE WHEN event_type = 'completed' THEN 1 ELSE 0 END) AS completed_count,
SUM(CASE WHEN event_type = 'snoozed' THEN 1 ELSE 0 END) AS snoozed_count,
SUM(CASE WHEN event_type = 'cancelled' THEN 1 ELSE 0 END) AS cancelled_count
FROM notification_events
GROUP BY template_id, variant
ORDER BY template_id, variant;`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query notification AB stats: %w", err)
	}
	defer rows.Close()

	var stats []NotificationABStat
	for rows.Next() {
		var st NotificationABStat
		if err := rows.Scan(&st.TemplateID, &st.Variant, &st.SentCount, &st.CompletedCount, &st.SnoozedCount, &st.CancelledCount); err != nil {
			return nil, fmt.Errorf("failed to scan AB stat row: %w", err)
		}
		if st.SentCount > 0 {
			st.ConversionRate = (float64(st.CompletedCount) / float64(st.SentCount)) * 100.0
		}
		stats = append(stats, st)
	}
	return stats, rows.Err()
}

// DB returns the underlying *sql.DB connection.
func (s *SQLStore) DB() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}

// Dialect returns the normalized SQL dialect ("postgres" or "sqlite3").
func (s *SQLStore) Dialect() string {
	if s == nil {
		return ""
	}
	return s.dialect
}

func (s *SQLStore) Close() error {
	return s.db.Close()
}

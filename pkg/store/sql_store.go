package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/littfed/max-bot-api-client/pkg/db"
	_ "modernc.org/sqlite"
)

// SQLStore implements a persistent database-backed storage for PostgreSQL and SQLite.
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
		if _, exists := s.getChannelLocked(ch.ChatID); !exists {
			_ = s.saveChannelLocked(ch)
		}
	}
}

// --- Channel Operations ---

func (s *SQLStore) SaveChannel(ch Channel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveChannelLocked(ch)
}

func (s *SQLStore) saveChannelLocked(ch Channel) error {
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

	_, err = s.db.Exec(query, ch.ChatID, ch.Title, ch.Type, ch.IsAdmin, string(permsJSON), ch.LastUpdated)
	return err
}

func (s *SQLStore) GetChannel(chatID int64) (Channel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getChannelLocked(chatID)
}

func (s *SQLStore) getChannelLocked(chatID int64) (Channel, bool) {
	query := `SELECT chat_id, title, type, is_admin, permissions, last_updated FROM channels WHERE chat_id = $1;`
	row := s.db.QueryRow(query, chatID)

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
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT chat_id, title, type, is_admin, permissions, last_updated FROM channels ORDER BY title ASC;`
	rows, err := s.db.Query(query)
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

	_, err = s.db.Exec(query, order.ID, order.UserID, order.UserName, string(itemsJSON), order.Total, order.DeliveryType, order.Address, order.Status, order.Verified, order.CreatedAt)
	return err
}

func (s *SQLStore) GetOrder(id string) (Order, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, user_id, user_name, items, total, delivery_type, address, status, verified, created_at FROM orders WHERE id = $1;`
	row := s.db.QueryRow(query, id)

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
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, user_id, user_name, items, total, delivery_type, address, status, verified, created_at FROM orders WHERE user_id = $1 ORDER BY created_at DESC;`
	rows, err := s.db.Query(query, userID)
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, user_id, user_name, items, total, delivery_type, address, status, verified, created_at FROM orders ORDER BY created_at DESC;`
	rows, err := s.db.Query(query)
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
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `UPDATE orders SET status = $1 WHERE id = $2;`
	res, err := s.db.Exec(query, status, id)
	if err != nil {
		return err
	}
	rowsAff, _ := res.RowsAffected()
	if rowsAff == 0 {
		return fmt.Errorf("order %q not found", id)
	}
	return nil
}

// --- Reaction Operations ---

func (s *SQLStore) ToggleReaction(postMID string, chatID int64, userID int64, emoji string) (map[string]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Fetch current reaction state
	query := `SELECT counts, user_votes FROM reactions WHERE post_mid = $1;`
	row := s.db.QueryRow(query, postMID)

	counts := make(map[string]int)
	userVotes := make(map[int64]string)

	var countsStr, votesStr string
	if err := row.Scan(&countsStr, &votesStr); err == nil {
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

	if _, err := s.db.Exec(saveQuery, postMID, chatID, string(cJSON), string(vJSON)); err != nil {
		return nil, err
	}

	resCounts := make(map[string]int)
	for k, v := range counts {
		resCounts[k] = v
	}
	return resCounts, nil
}

func (s *SQLStore) GetReactions(postMID string) (map[string]int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT counts FROM reactions WHERE post_mid = $1;`
	row := s.db.QueryRow(query, postMID)

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

	_, err := s.db.Exec(query, poll.ID, poll.ChatID, poll.MessageMID, poll.Question, string(optsJSON), string(votesJSON), poll.CreatedAt)
	return err
}

func (s *SQLStore) GetPoll(id string) (Poll, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, chat_id, message_mid, question, options, votes, created_at FROM polls WHERE id = $1;`
	row := s.db.QueryRow(query, id)

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
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `SELECT id, chat_id, message_mid, question, options, votes, created_at FROM polls WHERE id = $1;`
	row := s.db.QueryRow(query, pollID)

	var p Poll
	var optsStr, votesStr string
	if err := row.Scan(&p.ID, &p.ChatID, &p.MessageMID, &p.Question, &optsStr, &votesStr, &p.CreatedAt); err != nil {
		return nil, nil, fmt.Errorf("poll %s not found", pollID)
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
	if _, err := s.db.Exec(updateQuery, string(vJSON), pollID); err != nil {
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

	_, err := s.db.Exec(query, gw.ID, gw.ChatID, gw.Title, gw.PromoCode, gw.DiscountPercent, string(pJSON), string(wJSON), gw.Active, gw.CreatedAt)
	return err
}

func (s *SQLStore) GetGiveaway(id string) (Giveaway, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, chat_id, title, promo_code, discount_percent, participants, winners, active, created_at FROM giveaways WHERE id = $1;`
	row := s.db.QueryRow(query, id)

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
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `SELECT participants, active FROM giveaways WHERE id = $1;`
	row := s.db.QueryRow(query, id)

	var pStr string
	var active bool
	if err := row.Scan(&pStr, &active); err != nil {
		return fmt.Errorf("giveaway %s not found", id)
	}
	if !active {
		return fmt.Errorf("giveaway %s is already finished", id)
	}

	participants := make(map[int64]string)
	_ = json.Unmarshal([]byte(pStr), &participants)
	participants[userID] = userName

	pJSON, _ := json.Marshal(participants)
	updateQuery := `UPDATE giveaways SET participants = $1 WHERE id = $2;`
	_, err := s.db.Exec(updateQuery, string(pJSON), id)
	return err
}

func (s *SQLStore) DrawGiveawayWinners(id string, count int) ([]int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `SELECT id, chat_id, title, promo_code, discount_percent, participants, winners, active, created_at FROM giveaways WHERE id = $1;`
	row := s.db.QueryRow(query, id)

	var gw Giveaway
	var pStr, wStr string
	if err := row.Scan(&gw.ID, &gw.ChatID, &gw.Title, &gw.PromoCode, &gw.DiscountPercent, &pStr, &wStr, &gw.Active, &gw.CreatedAt); err != nil {
		return nil, fmt.Errorf("giveaway %s not found", id)
	}
	_ = json.Unmarshal([]byte(pStr), &gw.Participants)

	if len(gw.Participants) == 0 {
		return nil, errors.New("no participants in giveaway")
	}

	uids := make([]int64, 0, len(gw.Participants))
	for uid := range gw.Participants {
		uids = append(uids, uid)
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(uids), func(i, j int) {
		uids[i], uids[j] = uids[j], uids[i]
	})

	if count > len(uids) {
		count = len(uids)
	}
	winners := uids[:count]

	wJSON, _ := json.Marshal(winners)
	updateQuery := `UPDATE giveaways SET winners = $1, active = false WHERE id = $2;`
	if _, err := s.db.Exec(updateQuery, string(wJSON), id); err != nil {
		return nil, err
	}

	return winners, nil
}

// --- Media Artifact Operations ---

func (s *SQLStore) SaveMediaArtifact(artifact MediaArtifact) (*MediaArtifact, error) {
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
		err = s.db.QueryRow(query, artifact.UserID, artifact.ArtifactType, artifact.Title, artifact.Prompt, artifact.ContentText, artifact.MediaURL, string(metaJSON), artifact.CreatedAt).Scan(&artifact.ID)
		if err != nil {
			return nil, err
		}
	} else {
		query := `INSERT INTO media_history (user_id, artifact_type, title, prompt, content_text, media_url, metadata, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);`
		res, err := s.db.Exec(query, artifact.UserID, artifact.ArtifactType, artifact.Title, artifact.Prompt, artifact.ContentText, artifact.MediaURL, string(metaJSON), artifact.CreatedAt)
		if err != nil {
			return nil, err
		}
		artifact.ID, _ = res.LastInsertId()
	}

	return &artifact, nil
}

func (s *SQLStore) ListMediaArtifacts(userID int64, artifactType string, limit int) ([]MediaArtifact, error) {
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
		rows, err = s.db.Query(query, userID, artifactType, limit)
	} else {
		query := `SELECT id, user_id, artifact_type, title, prompt, content_text, media_url, metadata, created_at
FROM media_history WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2;`
		rows, err = s.db.Query(query, userID, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []MediaArtifact
	for rows.Next() {
		var a MediaArtifact
		var metaStr string
		if err := rows.Scan(&a.ID, &a.UserID, &a.ArtifactType, &a.Title, &a.Prompt, &a.ContentText, &a.MediaURL, &metaStr, &a.CreatedAt); err == nil {
			_ = json.Unmarshal([]byte(metaStr), &a.Metadata)
			result = append(result, a)
		}
	}
	return result, nil
}

func (s *SQLStore) Close() error {
	return s.db.Close()
}

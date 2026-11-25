package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// StoreState is the serialized snapshot of all dynamic application data.
type StoreState struct {
	Channels       map[int64]Channel    `json:"channels"`
	Orders         map[string]Order     `json:"orders"`
	Reactions      map[string]*Reaction `json:"reactions"`
	Polls          map[string]*Poll     `json:"polls"`
	Giveaways      map[string]*Giveaway `json:"giveaways"`
	MediaArtifacts []MediaArtifact      `json:"media_artifacts,omitempty"`
}

// Store provides thread-safe access and persistence for channels, orders, reactions, polls, and media artifacts.
// It transparently delegates to a SQL backend (PostgreSQL/SQLite) if configured, or falls back to JSON file / memory storage.
type Store struct {
	sql *SQLStore

	mu             sync.RWMutex
	filePath       string
	channels       map[int64]Channel
	orders         map[string]Order
	reactions      map[string]*Reaction
	polls          map[string]*Poll
	giveaways      map[string]*Giveaway
	mediaArtifacts []MediaArtifact
}

// New initializes or loads a Store from the specified JSON file path.
func New(filePath string) (*Store, error) {
	s := &Store{
		filePath:       filePath,
		channels:       make(map[int64]Channel),
		orders:         make(map[string]Order),
		reactions:      make(map[string]*Reaction),
		polls:          make(map[string]*Poll),
		giveaways:      make(map[string]*Giveaway),
		mediaArtifacts: make([]MediaArtifact, 0),
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

// NewWithSQL wraps an existing SQLStore in a Store.
func NewWithSQL(sqlStore *SQLStore) *Store {
	return &Store{
		sql: sqlStore,
	}
}

func configurePool(dbConn *sql.DB, dialect string) {
	if dialect == "sqlite" {
		dbConn.SetMaxOpenConns(1)
		dbConn.SetMaxIdleConns(1)
		dbConn.SetConnMaxLifetime(0)
		dbConn.SetConnMaxIdleTime(0)
		return
	}
	dbConn.SetMaxOpenConns(25)
	dbConn.SetMaxIdleConns(5)
	dbConn.SetConnMaxLifetime(15 * time.Minute)
	dbConn.SetConnMaxIdleTime(5 * time.Minute)
}

// OpenDatabase creates a *sql.DB for the given databaseURL (postgres:// or sqlite/file)
// with driver-appropriate connection pool defaults.
func OpenDatabase(databaseURL string) (*sql.DB, string, error) {
	trimmed := strings.TrimSpace(databaseURL)
	if strings.HasPrefix(trimmed, "postgres://") || strings.HasPrefix(trimmed, "postgresql://") {
		dbConn, err := sql.Open("pgx", trimmed)
		if err != nil {
			return nil, "", fmt.Errorf("failed to open postgres database: %w", err)
		}
		configurePool(dbConn, "postgres")
		return dbConn, "postgres", nil
	}

	dsn := trimmed
	if strings.HasPrefix(dsn, "sqlite://") {
		dsn = strings.TrimPrefix(dsn, "sqlite://")
	} else if strings.HasPrefix(dsn, "sqlite3://") {
		dsn = strings.TrimPrefix(dsn, "sqlite3://")
	}
	dbConn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open sqlite database: %w", err)
	}
	configurePool(dbConn, "sqlite")
	return dbConn, "sqlite", nil
}

// NewFromEnv initializes a Store using DATABASE_URL if present, falling back to filePath JSON.
func NewFromEnv(databaseURL string, fallbackPath string) (*Store, error) {
	if trimmed := strings.TrimSpace(databaseURL); trimmed != "" {
		dbConn, dialect, err := OpenDatabase(trimmed)
		if err != nil {
			log.Printf("[store] failed to connect to DATABASE_URL: %v, falling back to %s", err, fallbackPath)
			return New(fallbackPath)
		}

		configurePool(dbConn, dialect)

		sqlStore, err := NewSQLStore(dbConn, dialect)
		if err != nil {
			_ = dbConn.Close()
			log.Printf("[store] failed to initialize SQLStore (%s): %v, falling back to %s", dialect, err, fallbackPath)
			return New(fallbackPath)
		}

		log.Printf("[store] successfully connected to %s database and applied migrations", dialect)
		return NewWithSQL(sqlStore), nil
	}

	return New(fallbackPath)
}

// SQL returns the underlying SQLStore, or nil if using JSON/memory.
func (s *Store) SQL() *SQLStore {
	return s.sql
}

// Close closes the underlying storage backend if applicable.
func (s *Store) Close() error {
	if s.sql != nil {
		return s.sql.Close()
	}
	return nil
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
		dirty := false
		for k, ord := range s.orders {
			if sanitizeOrder(&ord) {
				s.orders[k] = ord
				dirty = true
			}
		}
		if dirty {
			_ = s.saveLocked()
		}
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
	if state.MediaArtifacts != nil {
		s.mediaArtifacts = state.MediaArtifacts
	}

	return nil
}

// saveLocked writes the state atomically to disk.
func (s *Store) saveLocked() error {
	if s.filePath == "" {
		return nil
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	state := StoreState{
		Channels:       s.channels,
		Orders:         s.orders,
		Reactions:      s.reactions,
		Polls:          s.polls,
		Giveaways:      s.giveaways,
		MediaArtifacts: s.mediaArtifacts,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal store state: %w", err)
	}

	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write tmp store file: %w", err)
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		return fmt.Errorf("failed to replace store file: %w", err)
	}

	return nil
}

// --- Channel Operations ---

func (s *Store) SaveChannel(ch Channel) error {
	if s.sql != nil {
		return s.sql.SaveChannel(ch)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	ch.LastUpdated = time.Now()
	s.channels[ch.ChatID] = ch
	return s.saveLocked()
}

func (s *Store) GetChannel(chatID int64) (Channel, bool) {
	if s.sql != nil {
		return s.sql.GetChannel(chatID)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	ch, ok := s.channels[chatID]
	return ch, ok
}

func (s *Store) ListChannels() []Channel {
	if s.sql != nil {
		return s.sql.ListChannels()
	}
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
	if s.sql != nil {
		return s.sql.CreateOrder(order)
	}
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
	if s.sql != nil {
		return s.sql.GetOrder(id)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	order, ok := s.orders[id]
	return order, ok
}

func (s *Store) ListOrdersByUser(userID int64) []Order {
	if s.sql != nil {
		return s.sql.ListOrdersByUser(userID)
	}
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
	if s.sql != nil {
		return s.sql.ListAllOrders()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := make([]Order, 0, len(s.orders))
	for _, ord := range s.orders {
		all = append(all, ord)
	}
	return all
}

func (s *Store) UpdateOrderStatus(id string, status string) error {
	if s.sql != nil {
		return s.sql.UpdateOrderStatus(id, status)
	}
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

func sanitizeOrder(ord *Order) bool {
	changed := false
	serviceMap := map[string]string{
		"srv_audit":   "Архитектурный аудит Go / MAX",
		"srv_review":  "Code Review & Рефакторинг",
		"srv_turnkey": "Разработка MAX-бота под ключ",
		"srv_coffee":  "Дегустация specialty кофе",
	}
	productMap := map[string]string{
		"gopher_hoodie":      "Худи Go Gopher Edition",
		"tech_tshirt":        "Футболка MAX Ecosystem",
		"barista_coffee":     "Кофемашина Barista 15bar",
		"pro_anc_headphones": "Наушники Pro ANC Wireless",
		"stickers_pack":      "Стикерпак Bot Masters",
		"go_patterns_book":   "Книга «Паттерны микросервисов в Go»",
	}

	for j := range ord.Items {
		if strings.Contains(ord.Items[j].Title, "?") || ord.Items[j].Title == "" {
			if title, ok := serviceMap[ord.Items[j].ID]; ok {
				ord.Items[j].Title = title
			} else if title, ok := productMap[ord.Items[j].ID]; ok {
				ord.Items[j].Title = title
			} else {
				ord.Items[j].Title = "Консультация по MAX Bot API"
			}
			changed = true
		}
	}
	if strings.Contains(ord.UserName, "?") || ord.UserName == "" {
		ord.UserName = "Федор В"
		changed = true
	}
	if strings.Contains(ord.Address, "?") {
		ord.Address = "Специалист: Алексей (Lead Go) | Время: Завтра, 14:00"
		changed = true
	}
	return changed
}

// --- Reaction Operations ---

func (s *Store) ToggleReaction(postMID string, chatID int64, userID int64, emoji string) (map[string]int, error) {
	if s.sql != nil {
		return s.sql.ToggleReaction(postMID, chatID, userID, emoji)
	}
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
		delete(rx.UserVotes, userID)
		if rx.Counts[emoji] > 0 {
			rx.Counts[emoji]--
		}
	} else {
		if hasVoted && rx.Counts[prevEmoji] > 0 {
			rx.Counts[prevEmoji]--
		}
		rx.UserVotes[userID] = emoji
		rx.Counts[emoji]++
	}

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
	if s.sql != nil {
		return s.sql.GetReactions(postMID)
	}
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

func clonePoll(p Poll) Poll {
	cp := p
	if p.Options != nil {
		cp.Options = make([]string, len(p.Options))
		copy(cp.Options, p.Options)
	}
	cp.Votes = make(map[int64]int, len(p.Votes))
	for k, v := range p.Votes {
		cp.Votes[k] = v
	}
	return cp
}

func (s *Store) CreatePoll(poll Poll) error {
	if s.sql != nil {
		return s.sql.CreatePoll(poll)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if poll.CreatedAt.IsZero() {
		poll.CreatedAt = time.Now()
	}
	cp := clonePoll(poll)
	s.polls[cp.ID] = &cp
	return s.saveLocked()
}

func (s *Store) GetPoll(id string) (Poll, bool) {
	if s.sql != nil {
		return s.sql.GetPoll(id)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	poll, ok := s.polls[id]
	if !ok {
		return Poll{}, false
	}
	return clonePoll(*poll), true
}

func (s *Store) VotePoll(pollID string, userID int64, optionIdx int) ([]int, []int, error) {
	if s.sql != nil {
		return s.sql.VotePoll(pollID, userID, optionIdx)
	}
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

func cloneGiveaway(gw Giveaway) Giveaway {
	cp := gw
	cp.Participants = make(map[int64]string, len(gw.Participants))
	for k, v := range gw.Participants {
		cp.Participants[k] = v
	}
	if gw.Winners != nil {
		cp.Winners = make([]int64, len(gw.Winners))
		copy(cp.Winners, gw.Winners)
	}
	return cp
}

func (s *Store) CreateGiveaway(gw Giveaway) error {
	if s.sql != nil {
		return s.sql.CreateGiveaway(gw)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if gw.CreatedAt.IsZero() {
		gw.CreatedAt = time.Now()
	}
	cp := cloneGiveaway(gw)
	cp.Active = true
	s.giveaways[cp.ID] = &cp
	return s.saveLocked()
}

func (s *Store) GetGiveaway(id string) (Giveaway, bool) {
	if s.sql != nil {
		return s.sql.GetGiveaway(id)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	gw, ok := s.giveaways[id]
	if !ok {
		return Giveaway{}, false
	}
	return cloneGiveaway(*gw), true
}

func (s *Store) AddGiveawayParticipant(id string, userID int64, userName string) error {
	if s.sql != nil {
		return s.sql.AddGiveawayParticipant(id, userID, userName)
	}
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
	if s.sql != nil {
		return s.sql.DrawGiveawayWinners(id, count)
	}
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

// --- Media Artifact Operations ---

func (s *Store) SaveMediaArtifact(artifact MediaArtifact) (*MediaArtifact, error) {
	if s.sql != nil {
		return s.sql.SaveMediaArtifact(artifact)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if artifact.CreatedAt.IsZero() {
		artifact.CreatedAt = time.Now()
	}
	if artifact.Metadata == nil {
		artifact.Metadata = make(map[string]any)
	}
	artifact.ID = int64(len(s.mediaArtifacts) + 1)
	s.mediaArtifacts = append(s.mediaArtifacts, artifact)
	_ = s.saveLocked()
	return &artifact, nil
}

func (s *Store) ListMediaArtifacts(userID int64, artifactType string, limit int) ([]MediaArtifact, error) {
	if s.sql != nil {
		return s.sql.ListMediaArtifacts(userID, artifactType, limit)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var res []MediaArtifact
	for i := len(s.mediaArtifacts) - 1; i >= 0; i-- {
		a := s.mediaArtifacts[i]
		if a.UserID == userID {
			if artifactType == "" || artifactType == "all" || a.ArtifactType == artifactType {
				res = append(res, a)
				if len(res) >= limit {
					break
				}
			}
		}
	}
	return res, nil
}

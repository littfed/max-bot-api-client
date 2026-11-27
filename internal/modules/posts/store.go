package posts

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ErrNotFound is returned when a requested profile or draft does not exist or does not belong to the user.
var ErrNotFound = errors.New("posts: not found")

// Repository defines persistence operations for Content Studio brand profiles and post drafts.
type Repository interface {
	GetDefaultProfile(ctx context.Context, userID int64) (*BrandProfile, error)
	ListProfiles(ctx context.Context, userID int64) ([]BrandProfile, error)
	UpsertProfile(ctx context.Context, p *BrandProfile) error
	CreateDraft(ctx context.Context, d *PostDraft) error
	GetDraft(ctx context.Context, userID, draftID int64) (*PostDraft, error)
	ListDrafts(ctx context.Context, userID int64) ([]PostDraft, error)
	UpdateDraft(ctx context.Context, d *PostDraft) error
}

// PGStore implements Repository backed by PostgreSQL.
type PGStore struct {
	db *sql.DB
}

// NewPGStore creates a new PostgreSQL-backed Repository.
func NewPGStore(db *sql.DB) *PGStore {
	return &PGStore{db: db}
}

func (s *PGStore) GetDefaultProfile(ctx context.Context, userID int64) (*BrandProfile, error) {
	const q = `
		SELECT id, user_id, name, niche, target_audience, tone_of_voice, forbidden_words,
		       default_platforms, is_default, created_at, updated_at
		FROM brand_profiles
		WHERE user_id = $1
		ORDER BY is_default DESC, updated_at DESC, id DESC
		LIMIT 1`

	p, err := scanProfile(s.db.QueryRowContext(ctx, q, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("posts: get default profile: %w", err)
	}
	return p, nil
}

func (s *PGStore) ListProfiles(ctx context.Context, userID int64) ([]BrandProfile, error) {
	const q = `
		SELECT id, user_id, name, niche, target_audience, tone_of_voice, forbidden_words,
		       default_platforms, is_default, created_at, updated_at
		FROM brand_profiles
		WHERE user_id = $1
		ORDER BY is_default DESC, updated_at DESC, id DESC`

	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("posts: list profiles: %w", err)
	}
	defer rows.Close()

	var profiles []BrandProfile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, fmt.Errorf("posts: scan profile: %w", err)
		}
		profiles = append(profiles, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("posts: iterate profiles: %w", err)
	}
	return profiles, nil
}

func (s *PGStore) UpsertProfile(ctx context.Context, p *BrandProfile) error {
	if p == nil {
		return errors.New("posts: profile cannot be nil")
	}

	platformsJSON, err := marshalJSONSlice(p.DefaultPlatforms)
	if err != nil {
		return fmt.Errorf("posts: marshal default_platforms: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("posts: begin tx: %w", err)
	}
	defer tx.Rollback()

	if p.IsDefault {
		if _, err := tx.ExecContext(ctx, `UPDATE brand_profiles SET is_default = FALSE WHERE user_id = $1`, p.UserID); err != nil {
			return fmt.Errorf("posts: clear default profiles: %w", err)
		}
	}

	if p.ID == 0 {
		const ins = `
			INSERT INTO brand_profiles (
				user_id, name, niche, target_audience, tone_of_voice, forbidden_words,
				default_platforms, is_default
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id, created_at, updated_at`
		if err := tx.QueryRowContext(
			ctx, ins,
			p.UserID, p.Name, p.Niche, p.TargetAudience, p.ToneOfVoice, p.ForbiddenWords,
			platformsJSON, p.IsDefault,
		).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return fmt.Errorf("posts: insert profile: %w", err)
		}
	} else {
		const upd = `
			UPDATE brand_profiles
			SET name = $3,
			    niche = $4,
			    target_audience = $5,
			    tone_of_voice = $6,
			    forbidden_words = $7,
			    default_platforms = $8,
			    is_default = $9,
			    updated_at = NOW()
			WHERE id = $1 AND user_id = $2
			RETURNING created_at, updated_at`
		err := tx.QueryRowContext(
			ctx, upd,
			p.ID, p.UserID, p.Name, p.Niche, p.TargetAudience, p.ToneOfVoice, p.ForbiddenWords,
			platformsJSON, p.IsDefault,
		).Scan(&p.CreatedAt, &p.UpdatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("posts: update profile: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("posts: commit profile: %w", err)
	}
	return nil
}

func (s *PGStore) CreateDraft(ctx context.Context, d *PostDraft) error {
	if d == nil {
		return errors.New("posts: draft cannot be nil")
	}
	if d.Status == "" {
		d.Status = DraftStatusIdea
	}

	platformsJSON, err := marshalJSONSlice(d.TargetPlatforms)
	if err != nil {
		return fmt.Errorf("posts: marshal target_platforms: %w", err)
	}
	hooksJSON, err := marshalJSONSlice(d.Hooks)
	if err != nil {
		return fmt.Errorf("posts: marshal hooks: %w", err)
	}
	variantsJSON, err := marshalJSONSlice(d.Variants)
	if err != nil {
		return fmt.Errorf("posts: marshal variants: %w", err)
	}
	imageHistoryJSON, err := marshalJSONSlice(d.ImageHistory)
	if err != nil {
		return fmt.Errorf("posts: marshal image_history: %w", err)
	}
	textHistoryJSON, err := marshalJSONSlice(d.TextHistory)
	if err != nil {
		return fmt.Errorf("posts: marshal text_history: %w", err)
	}

	const q = `
		INSERT INTO post_drafts (
			user_id, profile_id, status, raw_idea, source_image_url, vision_summary,
			goal, tone, target_platforms, hooks, selected_hook_id,
			image_prompt, generated_image_url, text_model, image_history, text_history, variants
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		RETURNING id, created_at, updated_at`

	err = s.db.QueryRowContext(
		ctx, q,
		d.UserID, d.ProfileID, d.Status, d.RawIdea, d.SourceImageURL, d.VisionSummary,
		d.Goal, d.Tone, platformsJSON, hooksJSON, d.SelectedHookID,
		d.ImagePrompt, d.GeneratedImageURL, d.TextModel, imageHistoryJSON, textHistoryJSON, variantsJSON,
	).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return fmt.Errorf("posts: create draft: %w", err)
	}
	return nil
}

func (s *PGStore) GetDraft(ctx context.Context, userID, draftID int64) (*PostDraft, error) {
	const q = `
		SELECT id, user_id, profile_id, status, raw_idea, source_image_url, vision_summary,
		       goal, tone, target_platforms, hooks, selected_hook_id,
		       image_prompt, generated_image_url, text_model, image_history, text_history, variants, created_at, updated_at
		FROM post_drafts
		WHERE id = $1 AND user_id = $2`

	d, err := scanDraft(s.db.QueryRowContext(ctx, q, draftID, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("posts: get draft: %w", err)
	}
	return d, nil
}

func (s *PGStore) ListDrafts(ctx context.Context, userID int64) ([]PostDraft, error) {
	const q = `
		SELECT id, user_id, profile_id, status, raw_idea, source_image_url, vision_summary,
		       goal, tone, target_platforms, hooks, selected_hook_id,
		       image_prompt, generated_image_url, text_model, image_history, text_history, variants, created_at, updated_at
		FROM post_drafts
		WHERE user_id = $1
		ORDER BY updated_at DESC, id DESC`

	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("posts: list drafts: %w", err)
	}
	defer rows.Close()

	var drafts []PostDraft
	for rows.Next() {
		d, err := scanDraft(rows)
		if err != nil {
			return nil, fmt.Errorf("posts: scan draft: %w", err)
		}
		drafts = append(drafts, *d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("posts: iterate drafts: %w", err)
	}
	return drafts, nil
}

func (s *PGStore) UpdateDraft(ctx context.Context, d *PostDraft) error {
	if d == nil {
		return errors.New("posts: draft cannot be nil")
	}

	platformsJSON, err := marshalJSONSlice(d.TargetPlatforms)
	if err != nil {
		return fmt.Errorf("posts: marshal target_platforms: %w", err)
	}
	hooksJSON, err := marshalJSONSlice(d.Hooks)
	if err != nil {
		return fmt.Errorf("posts: marshal hooks: %w", err)
	}
	variantsJSON, err := marshalJSONSlice(d.Variants)
	if err != nil {
		return fmt.Errorf("posts: marshal variants: %w", err)
	}
	imageHistoryJSON, err := marshalJSONSlice(d.ImageHistory)
	if err != nil {
		return fmt.Errorf("posts: marshal image_history: %w", err)
	}
	textHistoryJSON, err := marshalJSONSlice(d.TextHistory)
	if err != nil {
		return fmt.Errorf("posts: marshal text_history: %w", err)
	}

	const q = `
		UPDATE post_drafts
		SET profile_id = $3,
		    status = $4,
		    raw_idea = $5,
		    source_image_url = $6,
		    vision_summary = $7,
		    goal = $8,
		    tone = $9,
		    target_platforms = $10,
		    hooks = $11,
		    selected_hook_id = $12,
		    image_prompt = $13,
		    generated_image_url = $14,
		    text_model = $15,
		    image_history = $16,
		    text_history = $17,
		    variants = $18,
		    updated_at = NOW()
		WHERE id = $1 AND user_id = $2
		RETURNING updated_at`

	err = s.db.QueryRowContext(
		ctx, q,
		d.ID, d.UserID, d.ProfileID, d.Status, d.RawIdea, d.SourceImageURL, d.VisionSummary,
		d.Goal, d.Tone, platformsJSON, hooksJSON, d.SelectedHookID,
		d.ImagePrompt, d.GeneratedImageURL, d.TextModel, imageHistoryJSON, textHistoryJSON, variantsJSON,
	).Scan(&d.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("posts: update draft: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProfile(row rowScanner) (*BrandProfile, error) {
	var p BrandProfile
	var platformsRaw []byte
	err := row.Scan(
		&p.ID, &p.UserID, &p.Name, &p.Niche, &p.TargetAudience, &p.ToneOfVoice, &p.ForbiddenWords,
		&platformsRaw, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(platformsRaw) > 0 {
		if err := json.Unmarshal(platformsRaw, &p.DefaultPlatforms); err != nil {
			return nil, fmt.Errorf("unmarshal default_platforms: %w", err)
		}
	}
	return &p, nil
}

func scanDraft(row rowScanner) (*PostDraft, error) {
	var d PostDraft
	var profileID sql.NullInt64
	var platformsRaw, hooksRaw, imageHistoryRaw, textHistoryRaw, variantsRaw []byte
	err := row.Scan(
		&d.ID, &d.UserID, &profileID, &d.Status, &d.RawIdea, &d.SourceImageURL, &d.VisionSummary,
		&d.Goal, &d.Tone, &platformsRaw, &hooksRaw, &d.SelectedHookID,
		&d.ImagePrompt, &d.GeneratedImageURL, &d.TextModel, &imageHistoryRaw, &textHistoryRaw, &variantsRaw, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if profileID.Valid {
		v := profileID.Int64
		d.ProfileID = &v
	}
	if len(platformsRaw) > 0 {
		if err := json.Unmarshal(platformsRaw, &d.TargetPlatforms); err != nil {
			return nil, fmt.Errorf("unmarshal target_platforms: %w", err)
		}
	}
	if len(hooksRaw) > 0 {
		if err := json.Unmarshal(hooksRaw, &d.Hooks); err != nil {
			return nil, fmt.Errorf("unmarshal hooks: %w", err)
		}
	}
	if len(imageHistoryRaw) > 0 {
		if err := json.Unmarshal(imageHistoryRaw, &d.ImageHistory); err != nil {
			return nil, fmt.Errorf("unmarshal image_history: %w", err)
		}
	}
	if len(textHistoryRaw) > 0 {
		if err := json.Unmarshal(textHistoryRaw, &d.TextHistory); err != nil {
			return nil, fmt.Errorf("unmarshal text_history: %w", err)
		}
	}
	if len(variantsRaw) > 0 {
		if err := json.Unmarshal(variantsRaw, &d.Variants); err != nil {
			return nil, fmt.Errorf("unmarshal variants: %w", err)
		}
	}
	return &d, nil
}

func marshalJSONSlice[T any](v []T) ([]byte, error) {
	if v == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(v)
}

// MemoryStore is a thread-safe in-memory Repository implementation for tests and local runs.
type MemoryStore struct {
	mu          sync.RWMutex
	nextProfID  int64
	nextDraftID int64
	profiles    map[int64]BrandProfile
	drafts      map[int64]PostDraft
}

// NewMemoryStore creates a new in-memory Repository.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nextProfID:  1,
		nextDraftID: 1,
		profiles:    make(map[int64]BrandProfile),
		drafts:      make(map[int64]PostDraft),
	}
}

func (m *MemoryStore) GetDefaultProfile(_ context.Context, userID int64) (*BrandProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var candidates []BrandProfile
	for _, p := range m.profiles {
		if p.UserID == userID {
			candidates = append(candidates, cloneProfile(p))
		}
	}
	if len(candidates) == 0 {
		return nil, ErrNotFound
	}

	sortProfiles(candidates)
	res := candidates[0]
	return &res, nil
}

func (m *MemoryStore) ListProfiles(_ context.Context, userID int64) ([]BrandProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var res []BrandProfile
	for _, p := range m.profiles {
		if p.UserID == userID {
			res = append(res, cloneProfile(p))
		}
	}
	sortProfiles(res)
	return res, nil
}

func (m *MemoryStore) UpsertProfile(_ context.Context, p *BrandProfile) error {
	if p == nil {
		return errors.New("posts: profile cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()

	if p.IsDefault {
		for id, existing := range m.profiles {
			if existing.UserID == p.UserID && existing.IsDefault {
				existing.IsDefault = false
				m.profiles[id] = existing
			}
		}
	}

	if p.ID == 0 {
		p.ID = m.nextProfID
		m.nextProfID++
		p.CreatedAt = now
		p.UpdatedAt = now
		m.profiles[p.ID] = cloneProfile(*p)
		return nil
	}

	existing, ok := m.profiles[p.ID]
	if !ok || existing.UserID != p.UserID {
		return ErrNotFound
	}
	p.CreatedAt = existing.CreatedAt
	p.UpdatedAt = now
	m.profiles[p.ID] = cloneProfile(*p)
	return nil
}

func (m *MemoryStore) CreateDraft(_ context.Context, d *PostDraft) error {
	if d == nil {
		return errors.New("posts: draft cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	if d.Status == "" {
		d.Status = DraftStatusIdea
	}
	d.ID = m.nextDraftID
	m.nextDraftID++
	d.CreatedAt = now
	d.UpdatedAt = now
	m.drafts[d.ID] = cloneDraft(*d)
	return nil
}

func (m *MemoryStore) GetDraft(_ context.Context, userID, draftID int64) (*PostDraft, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	d, ok := m.drafts[draftID]
	if !ok || d.UserID != userID {
		return nil, ErrNotFound
	}
	res := cloneDraft(d)
	return &res, nil
}

func (m *MemoryStore) ListDrafts(_ context.Context, userID int64) ([]PostDraft, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var res []PostDraft
	for _, d := range m.drafts {
		if d.UserID == userID {
			res = append(res, cloneDraft(d))
		}
	}
	sort.Slice(res, func(i, j int) bool {
		if !res[i].UpdatedAt.Equal(res[j].UpdatedAt) {
			return res[i].UpdatedAt.After(res[j].UpdatedAt)
		}
		return res[i].ID > res[j].ID
	})
	return res, nil
}

func (m *MemoryStore) UpdateDraft(_ context.Context, d *PostDraft) error {
	if d == nil {
		return errors.New("posts: draft cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.drafts[d.ID]
	if !ok || existing.UserID != d.UserID {
		return ErrNotFound
	}

	d.CreatedAt = existing.CreatedAt
	d.UpdatedAt = time.Now().UTC()
	m.drafts[d.ID] = cloneDraft(*d)
	return nil
}

func sortProfiles(profiles []BrandProfile) {
	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].IsDefault != profiles[j].IsDefault {
			return profiles[i].IsDefault
		}
		if !profiles[i].UpdatedAt.Equal(profiles[j].UpdatedAt) {
			return profiles[i].UpdatedAt.After(profiles[j].UpdatedAt)
		}
		return profiles[i].ID > profiles[j].ID
	})
}

func cloneProfile(p BrandProfile) BrandProfile {
	if p.DefaultPlatforms != nil {
		cp := make([]string, len(p.DefaultPlatforms))
		copy(cp, p.DefaultPlatforms)
		p.DefaultPlatforms = cp
	}
	return p
}

func cloneDraft(d PostDraft) PostDraft {
	if d.ProfileID != nil {
		v := *d.ProfileID
		d.ProfileID = &v
	}
	if d.TargetPlatforms != nil {
		cp := make([]string, len(d.TargetPlatforms))
		copy(cp, d.TargetPlatforms)
		d.TargetPlatforms = cp
	}
	if d.Hooks != nil {
		cp := make([]PostHook, len(d.Hooks))
		copy(cp, d.Hooks)
		d.Hooks = cp
	}
	if d.ImageHistory != nil {
		cp := make([]ImageRevision, len(d.ImageHistory))
		copy(cp, d.ImageHistory)
		d.ImageHistory = cp
	}
	if d.TextHistory != nil {
		cp := make([]TextRevision, len(d.TextHistory))
		for i, r := range d.TextHistory {
			if r.Hashtags != nil {
				hc := make([]string, len(r.Hashtags))
				copy(hc, r.Hashtags)
				r.Hashtags = hc
			}
			cp[i] = r
		}
		d.TextHistory = cp
	}
	if d.Variants != nil {
		cp := make([]PostVariant, len(d.Variants))
		for i, v := range d.Variants {
			if v.Hashtags != nil {
				hc := make([]string, len(v.Hashtags))
				copy(hc, v.Hashtags)
				v.Hashtags = hc
			}
			cp[i] = v
		}
		d.Variants = cp
	}
	return d
}

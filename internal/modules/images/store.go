package images

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ErrProjectNotFound is returned when a requested project does not exist.
var ErrProjectNotFound = errors.New("images: project not found")

// Store defines persistence operations for image studio projects and revisions.
type Store interface {
	CreateProject(ctx context.Context, p *Project) error
	GetProject(ctx context.Context, id int64) (*Project, error)
	ListProjectsByUser(ctx context.Context, userID int64) ([]Project, error)
	UpdateProject(ctx context.Context, p *Project) error
	DeleteProject(ctx context.Context, id int64) error
	AddRevision(ctx context.Context, r *Revision) error
	ListRevisions(ctx context.Context, projectID int64) ([]Revision, error)
}

// SQLStore implements Store backed by PostgreSQL.
type SQLStore struct {
	db *sql.DB
}

// NewSQLStore creates a new PostgreSQL-backed Store.
func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

func (s *SQLStore) CreateProject(ctx context.Context, p *Project) error {
	if p == nil {
		return errors.New("images: project cannot be nil")
	}

	const q = `
		INSERT INTO image_projects (
			user_id, mode, preset_id, title,
			source_image_key, source_image_url,
			current_image_key, current_image_url,
			current_version, image_model, aspect_ratio, status, error_message
		) VALUES ($1, $2, NULLIF($3, ''), $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), $9, $10, $11, $12, NULLIF($13, ''))
		RETURNING id, created_at, updated_at`

	if err := s.db.QueryRowContext(
		ctx, q,
		p.UserID, p.Mode, p.PresetID, p.Title,
		p.SourceImageKey, p.SourceImageURL,
		p.CurrentImageKey, p.CurrentImageURL,
		p.CurrentVersion, p.ImageModel, p.AspectRatio, p.Status, p.ErrorMessage,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return fmt.Errorf("images: create project: %w", err)
	}
	return nil
}

func (s *SQLStore) GetProject(ctx context.Context, id int64) (*Project, error) {
	const q = `
		SELECT id, user_id, mode, COALESCE(preset_id, ''), title,
		       COALESCE(source_image_key, ''), COALESCE(source_image_url, ''),
		       COALESCE(current_image_key, ''), COALESCE(current_image_url, ''),
		       current_version, image_model, aspect_ratio, status, COALESCE(error_message, ''),
		       created_at, updated_at
		FROM image_projects
		WHERE id = $1`

	p, err := scanProject(s.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProjectNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("images: get project: %w", err)
	}
	return p, nil
}

func (s *SQLStore) ListProjectsByUser(ctx context.Context, userID int64) ([]Project, error) {
	const q = `
		SELECT id, user_id, mode, COALESCE(preset_id, ''), title,
		       COALESCE(source_image_key, ''), COALESCE(source_image_url, ''),
		       COALESCE(current_image_key, ''), COALESCE(current_image_url, ''),
		       current_version, image_model, aspect_ratio, status, COALESCE(error_message, ''),
		       created_at, updated_at
		FROM image_projects
		WHERE user_id = $1
		ORDER BY updated_at DESC, id DESC`

	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("images: list projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, fmt.Errorf("images: scan project: %w", err)
		}
		projects = append(projects, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("images: iterate projects: %w", err)
	}
	return projects, nil
}

func (s *SQLStore) UpdateProject(ctx context.Context, p *Project) error {
	if p == nil {
		return errors.New("images: project cannot be nil")
	}

	const q = `
		UPDATE image_projects
		SET mode = $2,
		    preset_id = NULLIF($3, ''),
		    title = $4,
		    source_image_key = NULLIF($5, ''),
		    source_image_url = NULLIF($6, ''),
		    current_image_key = NULLIF($7, ''),
		    current_image_url = NULLIF($8, ''),
		    current_version = $9,
		    image_model = $10,
		    aspect_ratio = $11,
		    status = $12,
		    error_message = NULLIF($13, ''),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at`

	err := s.db.QueryRowContext(
		ctx, q,
		p.ID, p.Mode, p.PresetID, p.Title,
		p.SourceImageKey, p.SourceImageURL,
		p.CurrentImageKey, p.CurrentImageURL,
		p.CurrentVersion, p.ImageModel, p.AspectRatio, p.Status, p.ErrorMessage,
	).Scan(&p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrProjectNotFound
	}
	if err != nil {
		return fmt.Errorf("images: update project: %w", err)
	}
	return nil
}

func (s *SQLStore) DeleteProject(ctx context.Context, id int64) error {
	const q = `DELETE FROM image_projects WHERE id = $1`
	res, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("images: delete project: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("images: delete project rows affected: %w", err)
	}
	if n == 0 {
		return ErrProjectNotFound
	}
	return nil
}

func (s *SQLStore) AddRevision(ctx context.Context, r *Revision) error {
	if r == nil {
		return errors.New("images: revision cannot be nil")
	}

	const q = `
		INSERT INTO image_revisions (
			project_id, version, prompt, action_type, image_key, image_url, model, pipeline_used
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at`

	if err := s.db.QueryRowContext(
		ctx, q,
		r.ProjectID, r.Version, r.Prompt, r.ActionType, r.ImageKey, r.ImageURL, r.Model, r.PipelineUsed,
	).Scan(&r.ID, &r.CreatedAt); err != nil {
		return fmt.Errorf("images: add revision: %w", err)
	}
	return nil
}

func (s *SQLStore) ListRevisions(ctx context.Context, projectID int64) ([]Revision, error) {
	const q = `
		SELECT id, project_id, version, prompt, action_type, image_key, image_url, model, pipeline_used, created_at
		FROM image_revisions
		WHERE project_id = $1
		ORDER BY version ASC, id ASC`

	rows, err := s.db.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("images: list revisions: %w", err)
	}
	defer rows.Close()

	var revisions []Revision
	for rows.Next() {
		var r Revision
		if err := rows.Scan(
			&r.ID, &r.ProjectID, &r.Version, &r.Prompt, &r.ActionType,
			&r.ImageKey, &r.ImageURL, &r.Model, &r.PipelineUsed, &r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("images: scan revision: %w", err)
		}
		revisions = append(revisions, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("images: iterate revisions: %w", err)
	}
	return revisions, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProject(row rowScanner) (*Project, error) {
	var p Project
	if err := row.Scan(
		&p.ID, &p.UserID, &p.Mode, &p.PresetID, &p.Title,
		&p.SourceImageKey, &p.SourceImageURL,
		&p.CurrentImageKey, &p.CurrentImageURL,
		&p.CurrentVersion, &p.ImageModel, &p.AspectRatio, &p.Status, &p.ErrorMessage,
		&p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &p, nil
}

// MemoryStore is a thread-safe in-memory implementation of Store for unit and API tests.
type MemoryStore struct {
	mu            sync.RWMutex
	nextProjectID int64
	nextRevID     int64
	projects      map[int64]*Project
	revisions     map[int64][]Revision
}

// NewMemoryStore creates a new in-memory Store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nextProjectID: 1,
		nextRevID:     1,
		projects:      make(map[int64]*Project),
		revisions:     make(map[int64][]Revision),
	}
}

func (m *MemoryStore) CreateProject(_ context.Context, p *Project) error {
	if p == nil {
		return errors.New("images: project cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	p.ID = m.nextProjectID
	m.nextProjectID++
	p.CreatedAt = now
	p.UpdatedAt = now
	if p.AspectRatio == "" {
		p.AspectRatio = "1:1"
	}
	if p.Status == "" {
		p.Status = StatusDraft
	}

	m.projects[p.ID] = cloneProject(p)
	return nil
}

func (m *MemoryStore) GetProject(_ context.Context, id int64) (*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.projects[id]
	if !ok {
		return nil, ErrProjectNotFound
	}
	return cloneProject(p), nil
}

func (m *MemoryStore) ListProjectsByUser(_ context.Context, userID int64) ([]Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var out []Project
	for _, p := range m.projects {
		if p.UserID == userID {
			out = append(out, *cloneProject(p))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].ID > out[j].ID
	})
	return out, nil
}

func (m *MemoryStore) UpdateProject(_ context.Context, p *Project) error {
	if p == nil {
		return errors.New("images: project cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.projects[p.ID]
	if !ok {
		return ErrProjectNotFound
	}

	p.CreatedAt = existing.CreatedAt
	p.UpdatedAt = time.Now().UTC()
	m.projects[p.ID] = cloneProject(p)
	return nil
}

func (m *MemoryStore) DeleteProject(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.projects[id]; !ok {
		return ErrProjectNotFound
	}
	delete(m.projects, id)
	delete(m.revisions, id)
	return nil
}

func (m *MemoryStore) AddRevision(_ context.Context, r *Revision) error {
	if r == nil {
		return errors.New("images: revision cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.projects[r.ProjectID]; !ok {
		return ErrProjectNotFound
	}

	for _, existing := range m.revisions[r.ProjectID] {
		if existing.Version == r.Version {
			return fmt.Errorf("images: duplicate revision version %d for project %d", r.Version, r.ProjectID)
		}
	}

	r.ID = m.nextRevID
	m.nextRevID++
	r.CreatedAt = time.Now().UTC()

	m.revisions[r.ProjectID] = append(m.revisions[r.ProjectID], *r)
	return nil
}

func (m *MemoryStore) ListRevisions(_ context.Context, projectID int64) ([]Revision, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	revs := m.revisions[projectID]
	if len(revs) == 0 {
		return nil, nil
	}
	out := make([]Revision, len(revs))
	copy(out, revs)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Version != out[j].Version {
			return out[i].Version < out[j].Version
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func cloneProject(p *Project) *Project {
	cp := *p
	if len(p.Revisions) > 0 {
		cp.Revisions = make([]Revision, len(p.Revisions))
		copy(cp.Revisions, p.Revisions)
	}
	return &cp
}

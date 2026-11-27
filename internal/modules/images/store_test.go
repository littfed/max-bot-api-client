package images

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestListPresets(t *testing.T) {
	presets := ListPresets()
	if len(presets) != 8 {
		t.Fatalf("expected 8 presets, got %d", len(presets))
	}

	var editCount, generateCount int
	seenIDs := make(map[string]bool)

	for _, p := range presets {
		if p.ID == "" {
			t.Fatal("preset has empty ID")
		}
		if seenIDs[p.ID] {
			t.Fatalf("duplicate preset ID: %q", p.ID)
		}
		seenIDs[p.ID] = true

		if p.Title == "" {
			t.Fatalf("preset %q has empty Title", p.ID)
		}
		if p.Description == "" {
			t.Fatalf("preset %q has empty Description", p.ID)
		}
		if p.PromptTemplate == "" {
			t.Fatalf("preset %q has empty PromptTemplate", p.ID)
		}
		if p.DefaultAspectRatio == "" {
			t.Fatalf("preset %q has empty DefaultAspectRatio", p.ID)
		}
		if p.ExamplePrompt == "" {
			t.Fatalf("preset %q has empty ExamplePrompt", p.ID)
		}
		if p.ExampleAfterURL == "" {
			t.Fatalf("preset %q has empty ExampleAfterURL", p.ID)
		}

		switch p.Mode {
		case ModeEdit:
			editCount++
			if p.ExampleBeforeURL == "" {
				t.Fatalf("edit preset %q must have non-empty ExampleBeforeURL", p.ID)
			}
		case ModeGenerate:
			generateCount++
			if p.ExampleBeforeURL != "" {
				t.Fatalf("generate preset %q must have empty ExampleBeforeURL, got %q", p.ID, p.ExampleBeforeURL)
			}
		default:
			t.Fatalf("preset %q has unexpected mode %q", p.ID, p.Mode)
		}
	}

	if editCount != 5 {
		t.Fatalf("expected 5 edit presets, got %d", editCount)
	}
	if generateCount != 3 {
		t.Fatalf("expected 3 generate presets, got %d", generateCount)
	}
}

func TestGetPreset(t *testing.T) {
	p, ok := GetPreset("bg_replace")
	if !ok {
		t.Fatal("expected GetPreset(\"bg_replace\") to return ok == true")
	}
	if p.ID != "bg_replace" || p.Mode != ModeEdit {
		t.Fatalf("unexpected preset returned: %+v", p)
	}
	if p.ExampleBeforeURL != "/presets/bg_replace_before.png" {
		t.Fatalf("unexpected ExampleBeforeURL: %q", p.ExampleBeforeURL)
	}
	if p.ExampleAfterURL != "/presets/bg_replace_after.png" {
		t.Fatalf("unexpected ExampleAfterURL: %q", p.ExampleAfterURL)
	}

	if _, ok := GetPreset("nonexistent_preset"); ok {
		t.Fatal("expected GetPreset(\"nonexistent_preset\") to return ok == false")
	}
}

func TestMemoryStore_CRUDAndRevisions(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	// 1. Create projects for user 1 and user 2
	p1 := &Project{
		UserID:      101,
		Mode:        ModeGenerate,
		PresetID:    "social_poster",
		Title:       "Запуск продукта",
		ImageModel:  "dall-e-3",
		AspectRatio: "16:9",
		Status:      StatusDraft,
	}
	if err := store.CreateProject(ctx, p1); err != nil {
		t.Fatalf("CreateProject p1 failed: %v", err)
	}
	if p1.ID == 0 {
		t.Fatal("expected non-zero ID after CreateProject")
	}
	if p1.CreatedAt.IsZero() || p1.UpdatedAt.IsZero() {
		t.Fatal("expected non-zero timestamps after CreateProject")
	}

	p2 := &Project{
		UserID:      202,
		Mode:        ModeEdit,
		PresetID:    "bg_replace",
		Title:       "Фото товара",
		AspectRatio: "1:1",
		Status:      StatusDraft,
	}
	if err := store.CreateProject(ctx, p2); err != nil {
		t.Fatalf("CreateProject p2 failed: %v", err)
	}

	// 2. GetProject & defensive copy check
	got, err := store.GetProject(ctx, p1.ID)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}
	if got.Title != p1.Title || got.UserID != 101 {
		t.Fatalf("unexpected project: %+v", got)
	}
	got.Title = "Mutated Title"
	gotAgain, err := store.GetProject(ctx, p1.ID)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}
	if gotAgain.Title != "Запуск продукта" {
		t.Fatalf("expected defensive copy, got mutated title %q", gotAgain.Title)
	}

	// 3. AddRevision & ListRevisions
	r1 := &Revision{
		ProjectID:    p1.ID,
		Version:      1,
		Prompt:       "Яркая обложка для анонса",
		ActionType:   ActionGenerate,
		ImageKey:     "images/101/1/v1.png",
		ImageURL:     "https://cdn.example.com/v1.png",
		Model:        "dall-e-3",
		PipelineUsed: PipelineGenerations,
	}
	if err := store.AddRevision(ctx, r1); err != nil {
		t.Fatalf("AddRevision r1 failed: %v", err)
	}
	if r1.ID == 0 || r1.CreatedAt.IsZero() {
		t.Fatal("expected non-zero ID and CreatedAt on revision")
	}

	r2 := &Revision{
		ProjectID:    p1.ID,
		Version:      2,
		Prompt:       "Сделай фон темнее",
		ActionType:   ActionRefine,
		ImageKey:     "images/101/1/v2.png",
		ImageURL:     "https://cdn.example.com/v2.png",
		Model:        "dall-e-3",
		PipelineUsed: PipelineEdits,
	}
	if err := store.AddRevision(ctx, r2); err != nil {
		t.Fatalf("AddRevision r2 failed: %v", err)
	}

	revs, err := store.ListRevisions(ctx, p1.ID)
	if err != nil {
		t.Fatalf("ListRevisions failed: %v", err)
	}
	if len(revs) != 2 || revs[0].Version != 1 || revs[1].Version != 2 {
		t.Fatalf("unexpected revisions: %+v", revs)
	}

	// 4. UpdateProject
	p1.CurrentImageKey = r2.ImageKey
	p1.CurrentImageURL = r2.ImageURL
	p1.CurrentVersion = 2
	p1.Status = StatusReady
	if err := store.UpdateProject(ctx, p1); err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}

	updated, err := store.GetProject(ctx, p1.ID)
	if err != nil {
		t.Fatalf("GetProject after update failed: %v", err)
	}
	if updated.CurrentVersion != 2 || updated.Status != StatusReady || updated.CurrentImageURL != r2.ImageURL {
		t.Fatalf("unexpected updated project: %+v", updated)
	}

	// 5. ListProjectsByUser
	u1Projects, err := store.ListProjectsByUser(ctx, 101)
	if err != nil {
		t.Fatalf("ListProjectsByUser failed: %v", err)
	}
	if len(u1Projects) != 1 || u1Projects[0].ID != p1.ID {
		t.Fatalf("expected 1 project for user 101, got %+v", u1Projects)
	}

	// 6. DeleteProject & ErrProjectNotFound
	if err := store.DeleteProject(ctx, p1.ID); err != nil {
		t.Fatalf("DeleteProject failed: %v", err)
	}
	if _, err := store.GetProject(ctx, p1.ID); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("expected ErrProjectNotFound after delete, got %v", err)
	}
	if err := store.DeleteProject(ctx, p1.ID); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("expected ErrProjectNotFound on second delete, got %v", err)
	}
	if err := store.UpdateProject(ctx, p1); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("expected ErrProjectNotFound on update after delete, got %v", err)
	}
	revsAfterDelete, err := store.ListRevisions(ctx, p1.ID)
	if err != nil {
		t.Fatalf("ListRevisions after delete failed: %v", err)
	}
	if len(revsAfterDelete) != 0 {
		t.Fatalf("expected 0 revisions after project delete, got %d", len(revsAfterDelete))
	}
}

type stubSQLDriver struct {
	queries *[]string
}

func (d *stubSQLDriver) Open(name string) (driver.Conn, error) {
	return &stubSQLConn{queries: d.queries}, nil
}

type stubSQLConn struct {
	queries *[]string
}

func (c *stubSQLConn) Prepare(query string) (driver.Stmt, error) {
	return &stubSQLStmt{query: query, queries: c.queries}, nil
}
func (c *stubSQLConn) Close() error              { return nil }
func (c *stubSQLConn) Begin() (driver.Tx, error) { return &stubSQLTx{}, nil }

func (c *stubSQLConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	*c.queries = append(*c.queries, query)
	now := time.Unix(1700000000, 0).UTC()
	switch {
	case strings.Contains(query, "INSERT INTO image_projects"):
		return &stubSQLRows{
			cols: []string{"id", "created_at", "updated_at"},
			data: [][]driver.Value{{int64(42), now, now}},
		}, nil
	case strings.Contains(query, "FROM image_projects"):
		return &stubSQLRows{
			cols: []string{
				"id", "user_id", "mode", "preset_id", "title",
				"source_image_key", "source_image_url",
				"current_image_key", "current_image_url",
				"current_version", "image_model", "aspect_ratio", "status", "error_message",
				"created_at", "updated_at",
			},
			data: [][]driver.Value{{
				int64(42), int64(7), ModeGenerate, "social_poster", "Poster",
				"", "",
				"images/7/42/v1.png", "https://cdn.example.com/v1.png",
				int64(1), "flux", "1:1", StatusReady, "",
				now, now,
			}},
		}, nil
	default:
		return &stubSQLRows{cols: []string{}}, nil
	}
}

type stubSQLTx struct{}

func (t *stubSQLTx) Commit() error   { return nil }
func (t *stubSQLTx) Rollback() error { return nil }

type stubSQLStmt struct {
	query   string
	queries *[]string
}

func (s *stubSQLStmt) Close() error  { return nil }
func (s *stubSQLStmt) NumInput() int { return -1 }
func (s *stubSQLStmt) Exec(args []driver.Value) (driver.Result, error) {
	*s.queries = append(*s.queries, s.query)
	return driver.RowsAffected(1), nil
}
func (s *stubSQLStmt) Query(args []driver.Value) (driver.Rows, error) {
	return &stubSQLRows{cols: []string{}}, nil
}

type stubSQLRows struct {
	cols []string
	data [][]driver.Value
	idx  int
}

func (r *stubSQLRows) Columns() []string { return r.cols }
func (r *stubSQLRows) Close() error      { return nil }
func (r *stubSQLRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.idx])
	r.idx++
	return nil
}

func TestSQLStore_CreateAndGetProject(t *testing.T) {
	var queries []string
	driverName := "images_stub_driver"
	sql.Register(driverName, &stubSQLDriver{queries: &queries})

	dbConn, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer dbConn.Close()

	st := NewSQLStore(dbConn)
	ctx := context.Background()

	p := &Project{
		UserID:      7,
		Mode:        ModeGenerate,
		PresetID:    "social_poster",
		Title:       "Poster",
		ImageModel:  "flux",
		AspectRatio: "1:1",
		Status:      StatusProcessing,
	}
	if err := st.CreateProject(ctx, p); err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}
	if p.ID != 42 || p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		t.Fatalf("unexpected project after CreateProject: %+v", p)
	}

	got, err := st.GetProject(ctx, 42)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}
	if got.ID != 42 || got.UserID != 7 || got.Status != StatusReady {
		t.Fatalf("unexpected project from GetProject: %+v", got)
	}
	if len(queries) != 2 {
		t.Fatalf("expected 2 SQL queries executed, got %d", len(queries))
	}
}

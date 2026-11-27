package storage

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestMemoryStorageCRUD(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStorage("https://cdn.example.com/")

	url, err := st.PutObject(ctx, "/images/1/2/v1.png", "image/png", []byte("png-bytes"))
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	if url != "https://cdn.example.com/images/1/2/v1.png" {
		t.Fatalf("unexpected url: %s", url)
	}

	data, ct, err := st.GetObject(ctx, "images/1/2/v1.png")
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	if ct != "image/png" || string(data) != "png-bytes" {
		t.Fatalf("unexpected data/ct: %q %q", string(data), ct)
	}

	// Verify returned slice is a defensive copy.
	data[0] = 'X'
	data2, _, err := st.GetObject(ctx, "images/1/2/v1.png")
	if err != nil {
		t.Fatalf("GetObject after mutation: %v", err)
	}
	if string(data2) != "png-bytes" {
		t.Fatalf("expected stored bytes to remain unmodified, got %q", string(data2))
	}

	if err := st.DeleteObject(ctx, "images/1/2/v1.png"); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
	if _, _, err := st.GetObject(ctx, "images/1/2/v1.png"); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("expected ErrObjectNotFound after delete, got %v", err)
	}
}

func TestMemoryStorageConcurrent(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStorage("/api/v1/images/assets")

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "images/concurrent/" + string(rune('a'+i)) + ".png"
			if _, err := st.PutObject(ctx, key, "image/png", []byte("ok")); err != nil {
				t.Errorf("PutObject %s: %v", key, err)
			}
			if _, _, err := st.GetObject(ctx, key); err != nil {
				t.Errorf("GetObject %s: %v", key, err)
			}
			if err := st.DeleteObject(ctx, key); err != nil {
				t.Errorf("DeleteObject %s: %v", key, err)
			}
		}(i)
	}
	wg.Wait()
}

func TestNewObjectStorageFromEnvFallback(t *testing.T) {
	t.Setenv("S3_ENDPOINT", "")
	t.Setenv("S3_BUCKET", "")
	t.Setenv("S3_ACCESS_KEY", "")
	t.Setenv("S3_SECRET_KEY", "")
	t.Setenv("S3_PUBLIC_URL", "")
	t.Setenv("S3_USE_SSL", "")

	st, err := NewObjectStorageFromEnv(context.Background())
	if err != nil {
		t.Fatalf("NewObjectStorageFromEnv: %v", err)
	}
	if _, ok := st.(*MemoryStorage); !ok {
		t.Fatalf("expected *MemoryStorage fallback, got %T", st)
	}

	url, err := st.PutObject(context.Background(), "images/1/2/v1.png", "image/png", []byte("data"))
	if err != nil {
		t.Fatalf("PutObject on fallback: %v", err)
	}
	if url != "/api/v1/images/assets/images/1/2/v1.png" {
		t.Fatalf("unexpected fallback url: %s", url)
	}
}

func TestNewS3StorageValidation(t *testing.T) {
	ctx := context.Background()

	if _, err := NewS3Storage(ctx, S3Config{Bucket: "images"}); err == nil {
		t.Fatal("expected error when S3 endpoint is empty")
	}
	if _, err := NewS3Storage(ctx, S3Config{Endpoint: "localhost:9000"}); err == nil {
		t.Fatal("expected error when S3 bucket is empty")
	}

	t.Setenv("S3_ENDPOINT", "localhost:9000")
	t.Setenv("S3_BUCKET", "images")
	t.Setenv("S3_USE_SSL", "not-a-bool")
	if _, err := NewObjectStorageFromEnv(ctx); err == nil {
		t.Fatal("expected error when S3_USE_SSL is invalid")
	}
}


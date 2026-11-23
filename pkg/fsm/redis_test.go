package fsm

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// Mini RESP server for testing RedisStorage without an external Redis instance
type mockRedisServer struct {
	ln   net.Listener
	data map[string]string
	mu   sync.Mutex
}

func startMockRedis(t *testing.T) *mockRedisServer {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock redis: %v", err)
	}

	srv := &mockRedisServer{
		ln:   ln,
		data: make(map[string]string),
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go srv.handleConn(conn)
		}
	}()

	return srv
}

func (s *mockRedisServer) handleConn(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		if !strings.HasPrefix(line, "*") {
			continue
		}
		var numArgs int
		fmt.Sscanf(line, "*%d", &numArgs)

		args := make([]string, 0, numArgs)
		for i := 0; i < numArgs; i++ {
			lenLine, _ := reader.ReadString('\n')
			var argLen int
			fmt.Sscanf(lenLine, "$%d", &argLen)
			buf := make([]byte, argLen+2)
			_, _ = reader.Read(buf)
			args = append(args, string(buf[:argLen]))
		}

		if len(args) == 0 {
			continue
		}

		cmd := strings.ToUpper(args[0])
		s.mu.Lock()
		switch cmd {
		case "PING":
			conn.Write([]byte("+PONG\r\n"))
		case "SET":
			key := args[1]
			val := args[2]
			s.data[key] = val
			conn.Write([]byte("+OK\r\n"))
		case "GET":
			key := args[1]
			val, ok := s.data[key]
			if !ok {
				conn.Write([]byte("$-1\r\n"))
			} else {
				conn.Write([]byte(fmt.Sprintf("$%d\r\n%s\r\n", len(val), val)))
			}
		case "DEL":
			for _, k := range args[1:] {
				delete(s.data, k)
			}
			conn.Write([]byte(":1\r\n"))
		default:
			conn.Write([]byte("+OK\r\n"))
		}
		s.mu.Unlock()
	}
}

func (s *mockRedisServer) Close() {
	_ = s.ln.Close()
}

func TestRedisStorageOperations(t *testing.T) {
	srv := startMockRedis(t)
	defer srv.Close()

	storage, err := NewRedisStorage(srv.ln.Addr().String(), RedisOptions{
		Prefix:  "test_fsm",
		TTL:     10 * time.Minute,
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create RedisStorage: %v", err)
	}

	ctx := context.Background()
	key := SessionKey{UserID: 1234, ChatID: 5678}

	// 1. Initial state
	st, err := storage.GetState(ctx, key)
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}
	if st != StateEmpty {
		t.Fatalf("expected empty state, got %v", st)
	}

	// 2. Set State
	if err := storage.SetState(ctx, key, "WAITING_EMAIL"); err != nil {
		t.Fatalf("SetState failed: %v", err)
	}
	st, err = storage.GetState(ctx, key)
	if err != nil || st != "WAITING_EMAIL" {
		t.Fatalf("expected WAITING_EMAIL, got %v", st)
	}

	// 3. Set Data
	data := map[string]any{"email": "bot@example.com"}
	if err := storage.SetData(ctx, key, data); err != nil {
		t.Fatalf("SetData failed: %v", err)
	}

	retrieved, err := storage.GetData(ctx, key)
	if err != nil || retrieved["email"] != "bot@example.com" {
		t.Fatalf("GetData returned unexpected data: %v", retrieved)
	}

	// 4. Reset
	if err := storage.Reset(ctx, key); err != nil {
		t.Fatalf("Reset failed: %v", err)
	}
	st, _ = storage.GetState(ctx, key)
	if st != StateEmpty {
		t.Fatalf("expected empty state after reset, got %v", st)
	}
}

func TestParseRedisURL(t *testing.T) {
	cases := []struct {
		url      string
		wantAddr string
		wantPass string
		wantDB   int
	}{
		{
			url:      "redis://127.0.0.1:6379",
			wantAddr: "127.0.0.1:6379",
			wantPass: "",
			wantDB:   0,
		},
		{
			url:      "redis://:mysecret@dokploy-redis:6380/2",
			wantAddr: "dokploy-redis:6380",
			wantPass: "mysecret",
			wantDB:   2,
		},
		{
			url:      "redis://default:complexpass@10.0.0.5:6379/1",
			wantAddr: "10.0.0.5:6379",
			wantPass: "complexpass",
			wantDB:   1,
		},
		{
			url:      "localhost:6379",
			wantAddr: "localhost:6379",
			wantPass: "",
			wantDB:   0,
		},
	}

	for _, c := range cases {
		addr, opts, err := ParseRedisURL(c.url)
		if err != nil {
			t.Fatalf("failed to parse url %s: %v", c.url, err)
		}
		if addr != c.wantAddr {
			t.Errorf("url %s: got addr %s, want %s", c.url, addr, c.wantAddr)
		}
		if opts.Password != c.wantPass {
			t.Errorf("url %s: got pass %s, want %s", c.url, opts.Password, c.wantPass)
		}
		if opts.DB != c.wantDB {
			t.Errorf("url %s: got db %d, want %d", c.url, opts.DB, c.wantDB)
		}
	}
}

func TestRedisStorageFromURLAndPing(t *testing.T) {
	srv := startMockRedis(t)
	defer srv.Close()

	redisURL := "redis://" + srv.ln.Addr().String() + "/0"
	storage, err := NewRedisStorageFromURL(redisURL, RedisOptions{TTL: 1 * time.Hour})
	if err != nil {
		t.Fatalf("failed to create redis storage from url: %v", err)
	}
	defer storage.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := storage.Ping(ctx); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	// Verify NewStorageFromEnv successfully picks up redis
	envStorage := NewStorageFromEnv(redisURL, 1*time.Hour)
	defer envStorage.Close()

	if _, ok := envStorage.(*RedisStorage); !ok {
		t.Fatalf("expected NewStorageFromEnv to return *RedisStorage, got %T", envStorage)
	}

	// Verify fallback to MemoryStorage on empty or invalid URL
	fallbackStorage := NewStorageFromEnv("", 1*time.Hour)
	defer fallbackStorage.Close()
	if _, ok := fallbackStorage.(*MemoryStorage); !ok {
		t.Fatalf("expected NewStorageFromEnv to fallback to *MemoryStorage, got %T", fallbackStorage)
	}
}

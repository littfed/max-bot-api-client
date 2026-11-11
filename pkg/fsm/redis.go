package fsm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RedisClient defines the minimum interface required for Redis operations.
// This allows drop-in compatibility with go-redis (redis.UniversalClient) or mock clients.
type RedisClient interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
}

// RedisStorage implements persistent distributed Storage backed by Redis.
type RedisStorage struct {
	client RedisClient
	prefix string
	ttl    time.Duration
}

// RedisOptions configures RedisStorage.
type RedisOptions struct {
	Prefix   string
	TTL      time.Duration
	Password string
	DB       int
	Timeout  time.Duration
}

// NewRedisStorage creates a RedisStorage using a built-in lightweight RESP client with zero external dependencies.
func NewRedisStorage(addr string, opts ...RedisOptions) (*RedisStorage, error) {
	var opt RedisOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	client := newRespClient(addr, opt.Password, opt.DB, opt.Timeout)
	return NewRedisStorageFromClient(client, opt.Prefix, opt.TTL), nil
}

// NewRedisStorageFromClient creates a RedisStorage wrapping any custom RedisClient implementation (e.g. go-redis/v9).
func NewRedisStorageFromClient(client RedisClient, prefix string, ttl time.Duration) *RedisStorage {
	if prefix == "" {
		prefix = "fsm"
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &RedisStorage{
		client: client,
		prefix: prefix,
		ttl:    ttl,
	}
}

func (r *RedisStorage) stateKey(k SessionKey) string {
	return fmt.Sprintf("%s:%d:%d:state", r.prefix, k.UserID, k.ChatID)
}

func (r *RedisStorage) dataKey(k SessionKey) string {
	return fmt.Sprintf("%s:%d:%d:data", r.prefix, k.UserID, k.ChatID)
}

// SetState stores the user's active state with TTL.
func (r *RedisStorage) SetState(ctx context.Context, key SessionKey, state State) error {
	k := r.stateKey(key)
	if state == StateEmpty {
		return r.client.Del(ctx, k)
	}
	return r.client.Set(ctx, k, string(state), r.ttl)
}

// GetState retrieves the active state for the user session.
func (r *RedisStorage) GetState(ctx context.Context, key SessionKey) (State, error) {
	val, err := r.client.Get(ctx, r.stateKey(key))
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			return StateEmpty, nil
		}
		return StateEmpty, err
	}
	return State(val), nil
}

// SetData stores serialized JSON session data with TTL.
func (r *RedisStorage) SetData(ctx context.Context, key SessionKey, data map[string]any) error {
	k := r.dataKey(key)
	if len(data) == 0 {
		return r.client.Del(ctx, k)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("fsm: failed to marshal redis data: %w", err)
	}
	return r.client.Set(ctx, k, string(raw), r.ttl)
}

// GetData retrieves and decodes stored JSON session data.
func (r *RedisStorage) GetData(ctx context.Context, key SessionKey) (map[string]any, error) {
	val, err := r.client.Get(ctx, r.dataKey(key))
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			return make(map[string]any), nil
		}
		return nil, err
	}
	result := make(map[string]any)
	if val == "" || val == "{}" {
		return result, nil
	}
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		return nil, fmt.Errorf("fsm: failed to unmarshal redis data: %w", err)
	}
	return result, nil
}

// UpdateData mutates the session data atomically.
func (r *RedisStorage) UpdateData(ctx context.Context, key SessionKey, fn func(data map[string]any) error) error {
	data, err := r.GetData(ctx, key)
	if err != nil {
		data = make(map[string]any)
	}
	if err := fn(data); err != nil {
		return err
	}
	return r.SetData(ctx, key, data)
}

// Reset deletes both state and data keys for the given session.
func (r *RedisStorage) Reset(ctx context.Context, key SessionKey) error {
	return r.client.Del(ctx, r.stateKey(key), r.dataKey(key))
}

// ErrKeyNotFound indicates a Redis key does not exist.
var ErrKeyNotFound = errors.New("fsm: redis key not found")

// respClient is a lightweight, thread-safe, pure Go RESP2 client.
type respClient struct {
	addr     string
	password string
	db       int
	timeout  time.Duration
	mu       sync.Mutex
	conn     net.Conn
	reader   *bufio.Reader
}

func newRespClient(addr string, password string, db int, timeout time.Duration) *respClient {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &respClient{
		addr:     addr,
		password: password,
		db:       db,
		timeout:  timeout,
	}
}

func (c *respClient) getConn() (net.Conn, *bufio.Reader, error) {
	if c.conn != nil {
		return c.conn, c.reader, nil
	}
	conn, err := net.DialTimeout("tcp", c.addr, c.timeout)
	if err != nil {
		return nil, nil, err
	}
	reader := bufio.NewReader(conn)
	c.conn = conn
	c.reader = reader

	// Handle AUTH
	if c.password != "" {
		if err := c.execRaw("AUTH", c.password); err != nil {
			_ = conn.Close()
			c.conn = nil
			return nil, nil, fmt.Errorf("redis auth failed: %w", err)
		}
	}
	// Handle SELECT DB
	if c.db > 0 {
		if err := c.execRaw("SELECT", strconv.Itoa(c.db)); err != nil {
			_ = conn.Close()
			c.conn = nil
			return nil, nil, fmt.Errorf("redis select db failed: %w", err)
		}
	}

	return c.conn, c.reader, nil
}

func (c *respClient) closeConn() {
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
		c.reader = nil
	}
}

func (c *respClient) execRaw(args ...string) error {
	cmd := formatRESPCommand(args...)
	_ = c.conn.SetDeadline(time.Now().Add(c.timeout))
	if _, err := c.conn.Write([]byte(cmd)); err != nil {
		return err
	}
	res, err := readRESP(c.reader)
	if err != nil {
		return err
	}
	if str, ok := res.(string); ok && strings.HasPrefix(str, "ERR") {
		return errors.New(str)
	}
	return nil
}

func (c *respClient) Get(ctx context.Context, key string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, r, err := c.getConn()
	if err != nil {
		return "", err
	}

	cmd := formatRESPCommand("GET", key)
	_ = conn.SetDeadline(time.Now().Add(c.timeout))
	if _, err := conn.Write([]byte(cmd)); err != nil {
		c.closeConn()
		return "", err
	}

	res, err := readRESP(r)
	if err != nil {
		c.closeConn()
		return "", err
	}

	if res == nil {
		return "", ErrKeyNotFound
	}

	str, ok := res.(string)
	if !ok {
		return "", fmt.Errorf("unexpected redis response type: %T", res)
	}
	return str, nil
}

func (c *respClient) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, r, err := c.getConn()
	if err != nil {
		return err
	}

	var cmd string
	if ttl > 0 {
		cmd = formatRESPCommand("SET", key, value, "EX", strconv.Itoa(int(ttl.Seconds())))
	} else {
		cmd = formatRESPCommand("SET", key, value)
	}

	_ = conn.SetDeadline(time.Now().Add(c.timeout))
	if _, err := conn.Write([]byte(cmd)); err != nil {
		c.closeConn()
		return err
	}

	res, err := readRESP(r)
	if err != nil {
		c.closeConn()
		return err
	}

	if str, ok := res.(string); ok && strings.HasPrefix(str, "ERR") {
		return errors.New(str)
	}
	return nil
}

func (c *respClient) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, r, err := c.getConn()
	if err != nil {
		return err
	}

	args := append([]string{"DEL"}, keys...)
	cmd := formatRESPCommand(args...)

	_ = conn.SetDeadline(time.Now().Add(c.timeout))
	if _, err := conn.Write([]byte(cmd)); err != nil {
		c.closeConn()
		return err
	}

	_, err = readRESP(r)
	if err != nil {
		c.closeConn()
		return err
	}
	return nil
}

func formatRESPCommand(args ...string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*%d\r\n", len(args)))
	for _, arg := range args {
		sb.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg))
	}
	return sb.String()
}

func readRESP(r *bufio.Reader) (any, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 2 {
		return nil, errors.New("empty RESP line")
	}
	line = strings.TrimSuffix(line, "\r\n")
	prefix := line[0]
	payload := line[1:]

	switch prefix {
	case '+': // Simple string
		return payload, nil
	case ':': // Integer
		return strconv.ParseInt(payload, 10, 64)
	case '-': // Error
		return "ERR " + payload, nil
	case '$': // Bulk string
		length, err := strconv.Atoi(payload)
		if err != nil {
			return nil, err
		}
		if length == -1 {
			return nil, nil // Null bulk string
		}
		buf := make([]byte, length+2) // include \r\n
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return string(buf[:length]), nil
	default:
		return nil, fmt.Errorf("unsupported RESP prefix: %c", prefix)
	}
}

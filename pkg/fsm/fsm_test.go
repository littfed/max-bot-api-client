package fsm

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/littfed/max-bot-api-client/pkg/bot"
	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func TestMemoryStorageLifecycle(t *testing.T) {
	storage := NewMemoryStorage(500 * time.Millisecond)
	defer storage.Close()

	ctx := context.Background()
	key := SessionKey{UserID: 100, ChatID: 200}

	// 1. Initial state
	st, err := storage.GetState(ctx, key)
	if err != nil || st != DefaultState {
		t.Fatalf("expected DefaultState, got %s (err: %v)", st, err)
	}

	// 2. Set state
	if err := storage.SetState(ctx, key, "WAIT_NAME"); err != nil {
		t.Fatalf("SetState failed: %v", err)
	}
	st, _ = storage.GetState(ctx, key)
	if st != "WAIT_NAME" {
		t.Fatalf("expected WAIT_NAME, got %s", st)
	}

	// 3. Set and get data
	data := map[string]any{"name": "Иван", "age": 28}
	if err := storage.SetData(ctx, key, data); err != nil {
		t.Fatalf("SetData failed: %v", err)
	}

	resData, err := storage.GetData(ctx, key)
	if err != nil || resData["name"] != "Иван" || resData["age"] != 28 {
		t.Fatalf("GetData mismatch: %+v", resData)
	}

	// 4. Update data
	err = storage.UpdateData(ctx, key, func(d map[string]any) error {
		d["city"] = "Москва"
		return nil
	})
	if err != nil {
		t.Fatalf("UpdateData failed: %v", err)
	}
	resData, _ = storage.GetData(ctx, key)
	if resData["city"] != "Москва" {
		t.Fatalf("expected city Москва, got %v", resData["city"])
	}

	// 5. Reset
	if err := storage.Reset(ctx, key); err != nil {
		t.Fatalf("Reset failed: %v", err)
	}
	st, _ = storage.GetState(ctx, key)
	if st != DefaultState {
		t.Fatalf("expected DefaultState after Reset, got %s", st)
	}
}

func TestMemoryStorageTTLExpiration(t *testing.T) {
	// Storage with 50ms TTL
	storage := NewMemoryStorage(50 * time.Millisecond)
	defer storage.Close()

	ctx := context.Background()
	key := SessionKey{UserID: 1, ChatID: 1}

	_ = storage.SetState(ctx, key, "TEMP_STEP")
	_ = storage.SetData(ctx, key, map[string]any{"temp": 123})

	time.Sleep(100 * time.Millisecond)

	st, _ := storage.GetState(ctx, key)
	if st != DefaultState {
		t.Fatalf("expected session to expire after TTL, got state %s", st)
	}
}

func TestMemoryStorageConcurrency(t *testing.T) {
	storage := NewMemoryStorage(time.Minute)
	defer storage.Close()

	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := SessionKey{UserID: int64(id), ChatID: 10}
			for j := 0; j < 50; j++ {
				_ = storage.SetState(ctx, key, State("STATE_STEP"))
				_ = storage.UpdateData(ctx, key, func(d map[string]any) error {
					d["counter"] = j
					return nil
				})
				_, _ = storage.GetState(ctx, key)
				_, _ = storage.GetData(ctx, key)
			}
		}(i)
	}

	wg.Wait()
}

func TestFSMContextAndRouterIntegration(t *testing.T) {
	storage := NewMemoryStorage(time.Minute)
	defer storage.Close()

	router := bot.NewRouter()
	router.Use(Middleware(storage))

	step1Handled := false
	step2Handled := false

	// Register state-filtered routes
	router.OnState("STEP_ONE", func(c *bot.Context) error {
		step1Handled = true
		fsmCtx := FromContext(c)
		_ = fsmCtx.Set("user_input", c.Text())
		_ = fsmCtx.SetState("STEP_TWO")
		return nil
	})

	router.OnState("STEP_TWO", func(c *bot.Context) error {
		step2Handled = true
		return nil
	})

	u := &maxbot.Update{
		UpdateType: maxbot.UpdateTypeMessageCreated,
		Message: &maxbot.Message{
			Sender: &maxbot.User{UserID: 777},
			Recipient: &maxbot.MessageRecipient{
				ChatID: 888,
			},
			Body: maxbot.MessageBody{Text: "Привет"},
		},
	}

	// 1. Manually set state to STEP_ONE
	key := SessionKey{UserID: 777, ChatID: 888}
	_ = storage.SetState(context.Background(), key, "STEP_ONE")

	c1 := bot.NewContext(context.Background(), nil, u)
	if err := router.Handle(c1); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if !step1Handled {
		t.Fatalf("STEP_ONE handler was not invoked")
	}

	// Verify state transitioned to STEP_TWO
	st, _ := storage.GetState(context.Background(), key)
	if st != "STEP_TWO" {
		t.Fatalf("expected state STEP_TWO, got %s", st)
	}

	// 2. Dispatch second update (should hit STEP_TWO)
	c2 := bot.NewContext(context.Background(), nil, u)
	if err := router.Handle(c2); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	if !step2Handled {
		t.Fatalf("STEP_TWO handler was not invoked")
	}
}

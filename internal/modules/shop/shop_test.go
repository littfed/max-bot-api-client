package shop

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

func TestShop_CatalogOrdersAndPayment(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := New(nil)

	if m.ID() != "shop" || m.Title() == "" {
		t.Fatalf("unexpected module metadata: id=%q title=%q", m.ID(), m.Title())
	}

	catalog, err := m.GetCatalog(ctx)
	if err != nil || len(catalog) < 4 {
		t.Fatalf("GetCatalog failed: len=%d err=%v", len(catalog), err)
	}

	p, err := m.GetProduct(ctx, "prod_guide")
	if err != nil || p.ID != "prod_guide" {
		t.Fatalf("GetProduct failed: p=%+v err=%v", p, err)
	}
	if _, err := m.GetProduct(ctx, "prod_missing"); err == nil {
		t.Fatal("expected error for missing product")
	}

	order, err := m.CreateOrder(ctx, 501, "prod_guide")
	if err != nil || order.Status != "pending" {
		t.Fatalf("CreateOrder failed: order=%+v err=%v", order, err)
	}
	if _, err := m.CreateOrder(ctx, 501, "prod_missing"); err == nil {
		t.Fatal("expected error for missing product in CreateOrder")
	}

	gotOrder, err := m.GetOrder(ctx, order.ID)
	if err != nil || gotOrder.ID != order.ID {
		t.Fatalf("GetOrder failed: got=%+v err=%v", gotOrder, err)
	}
	orders, err := m.GetUserOrders(ctx, 501)
	if err != nil || len(orders) != 1 {
		t.Fatalf("GetUserOrders failed: len=%d err=%v", len(orders), err)
	}

	simRes, err := m.SimulatePayment(ctx, order.ID, "paid")
	if err != nil || simRes.Status != "paid" || simRes.DeliveryPayload == "" {
		t.Fatalf("SimulatePayment(paid) failed: res=%+v err=%v", simRes, err)
	}
	if _, err := m.SimulatePayment(ctx, order.ID, "unknown_action"); err == nil {
		t.Fatal("expected error for unsupported action in SimulatePayment")
	}
}

func TestShop_ConcurrentOrdersAndCallbacks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m := New(nil)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if _, err := m.CreateOrder(ctx, int64(1000+id), "prod_prompts"); err != nil {
				t.Errorf("concurrent CreateOrder %d failed: %v", id, err)
			}
		}(i)
	}
	wg.Wait()

	user := &store.User{ID: 777}
	res, err := m.HandleCallback(ctx, user, &core.State{}, &core.IncomingCallback{Data: fmt.Sprintf("shop:buy:%s", "prod_guide")})
	if err != nil || res == nil || !res.Handled || res.Text == "" {
		t.Fatalf("HandleCallback(shop:buy) failed: res=%+v err=%v", res, err)
	}
}

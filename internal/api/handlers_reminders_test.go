package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestRemindersAPI_Lifecycle(t *testing.T) {
	ts := setupTestAPIServer(t)
	client := ts.Client()

	// 1. Create a reminder (POST /api/v1/reminders)
	remindAtStr := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
	createBody := map[string]any{
		"title":           "Купить кофе в офис",
		"description":     "Зерновой, средней обжарки",
		"remind_at":       remindAtStr,
		"recurrence_rule": "daily",
	}
	bodyBytes, _ := json.Marshal(createBody)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/reminders", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Init-Data", validMAXInitData)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/reminders failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201 Created, got %d", resp.StatusCode)
	}

	var createResp struct {
		Reminder struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
		} `json:"reminder"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if createResp.Reminder.ID == 0 || createResp.Reminder.Title != "Купить кофе в офис" {
		t.Fatalf("unexpected created reminder: %+v", createResp)
	}
	remID := createResp.Reminder.ID

	// 2. List reminders (GET /api/v1/reminders)
	getReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/reminders?status=active", nil)
	getReq.Header.Set("X-Init-Data", validMAXInitData)

	getResp, err := client.Do(getReq)
	if err != nil {
		t.Fatalf("GET /api/v1/reminders failed: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", getResp.StatusCode)
	}

	var listResp struct {
		Reminders []struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
		} `json:"reminders"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode list: %v", err)
	}
	if len(listResp.Reminders) != 1 || listResp.Reminders[0].ID != remID {
		t.Fatalf("expected 1 reminder with id %d, got %+v", remID, listResp)
	}

	// 3. Action snooze (PATCH /api/v1/reminders/{id}/action)
	snoozeBody, _ := json.Marshal(map[string]any{
		"action":  "snooze",
		"minutes": 30,
	})
	patchReq, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/api/v1/reminders/%d/action", ts.URL, remID), bytes.NewReader(snoozeBody))
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq.Header.Set("X-Init-Data", validMAXInitData)

	patchResp, err := client.Do(patchReq)
	if err != nil {
		t.Fatalf("PATCH action snooze failed: %v", err)
	}
	defer patchResp.Body.Close()

	if patchResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", patchResp.StatusCode)
	}

	// 4. Action complete (PATCH /api/v1/reminders/{id}/action)
	completeBody, _ := json.Marshal(map[string]any{
		"action": "complete",
	})
	compReq, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/api/v1/reminders/%d/action", ts.URL, remID), bytes.NewReader(completeBody))
	compReq.Header.Set("Content-Type", "application/json")
	compReq.Header.Set("X-Init-Data", validMAXInitData)

	compResp, err := client.Do(compReq)
	if err != nil {
		t.Fatalf("PATCH action complete failed: %v", err)
	}
	defer compResp.Body.Close()

	if compResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", compResp.StatusCode)
	}

	// 5. Simulate demo reminder (POST /api/v1/reminders/simulate)
	simReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/reminders/simulate", nil)
	simReq.Header.Set("X-Init-Data", validMAXInitData)

	simResp, err := client.Do(simReq)
	if err != nil {
		t.Fatalf("POST /api/v1/reminders/simulate failed: %v", err)
	}
	defer simResp.Body.Close()

	if simResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", simResp.StatusCode)
	}

	// 6. Delete reminder (DELETE /api/v1/reminders/{id})
	delReq, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/reminders/%d", ts.URL, remID), nil)
	delReq.Header.Set("X-Init-Data", validMAXInitData)

	delResp, err := client.Do(delReq)
	if err != nil {
		t.Fatalf("DELETE /api/v1/reminders/{id} failed: %v", err)
	}
	defer delResp.Body.Close()

	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", delResp.StatusCode)
	}

	// 7. Admin A/B stats (GET /api/v1/admin/notifications/ab-stats)
	adminReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/admin/notifications/ab-stats", nil)
	adminReq.Header.Set("X-Init-Data", validAdminInitData)

	adminResp, err := client.Do(adminReq)
	if err != nil {
		t.Fatalf("GET /api/v1/admin/notifications/ab-stats failed: %v", err)
	}
	defer adminResp.Body.Close()

	if adminResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", adminResp.StatusCode)
	}
}

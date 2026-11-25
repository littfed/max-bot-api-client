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

func TestRemindersAPI_AuthzAndValidation(t *testing.T) {
	ts := setupTestAPIServer(t)
	client := ts.Client()

	// 1. Invalid payload -> 400
	badReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/reminders", bytes.NewReader([]byte(`{"title":""}`)))
	badReq.Header.Set("Content-Type", "application/json")
	badReq.Header.Set("X-Init-Data", validMAXInitData)
	badResp, err := client.Do(badReq)
	if err != nil {
		t.Fatalf("POST invalid failed: %v", err)
	}
	badResp.Body.Close()
	if badResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty title, got %d", badResp.StatusCode)
	}

	// 2. Create reminder as validMAXInitData
	createBody, _ := json.Marshal(map[string]any{
		"title":     "Owner check",
		"remind_at": time.Now().Add(time.Hour).Format(time.RFC3339),
	})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/reminders", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Init-Data", validMAXInitData)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST create failed: %v", err)
	}
	var created struct {
		Reminder struct {
			ID int64 `json:"id"`
		} `json:"reminder"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	// 3. Cross-user action & delete -> 403 Forbidden
	actBody, _ := json.Marshal(map[string]any{"action": "complete"})
	idorPatch, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/api/v1/reminders/%d/action", ts.URL, created.Reminder.ID), bytes.NewReader(actBody))
	idorPatch.Header.Set("Content-Type", "application/json")
	idorPatch.Header.Set("X-Init-Data", validAdminInitData)
	idorPatchResp, err := client.Do(idorPatch)
	if err != nil {
		t.Fatalf("PATCH IDOR failed: %v", err)
	}
	idorPatchResp.Body.Close()
	if idorPatchResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on cross-user PATCH, got %d", idorPatchResp.StatusCode)
	}

	idorDel, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/reminders/%d", ts.URL, created.Reminder.ID), nil)
	idorDel.Header.Set("X-Init-Data", validAdminInitData)
	idorDelResp, err := client.Do(idorDel)
	if err != nil {
		t.Fatalf("DELETE IDOR failed: %v", err)
	}
	idorDelResp.Body.Close()
	if idorDelResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on cross-user DELETE, got %d", idorDelResp.StatusCode)
	}

	// 4. Non-existent ID -> 404 Not Found
	notFoundDel, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/reminders/999999", nil)
	notFoundDel.Header.Set("X-Init-Data", validMAXInitData)
	nfResp, err := client.Do(notFoundDel)
	if err != nil {
		t.Fatalf("DELETE 404 failed: %v", err)
	}
	nfResp.Body.Close()
	if nfResp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", nfResp.StatusCode)
	}

	// 5. Repeated completion -> 409 Conflict
	comp1, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/api/v1/reminders/%d/action", ts.URL, created.Reminder.ID), bytes.NewReader(actBody))
	comp1.Header.Set("Content-Type", "application/json")
	comp1.Header.Set("X-Init-Data", validMAXInitData)
	r1, _ := client.Do(comp1)
	r1.Body.Close()
	if r1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on first complete, got %d", r1.StatusCode)
	}

	comp2, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/api/v1/reminders/%d/action", ts.URL, created.Reminder.ID), bytes.NewReader(actBody))
	comp2.Header.Set("Content-Type", "application/json")
	comp2.Header.Set("X-Init-Data", validMAXInitData)
	r2, _ := client.Do(comp2)
	r2.Body.Close()
	if r2.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on repeated complete, got %d", r2.StatusCode)
	}
}

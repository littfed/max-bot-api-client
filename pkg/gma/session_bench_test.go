package gma

import (
	"testing"
	"time"
)

func BenchmarkSessionManager_AddMessage(b *testing.B) {
	sm := NewSessionManager(12, 1*time.Hour)
	userID := int64(98765)
	msg := NewTextMessage(RoleUser, "Тестовый вопрос для бенчмарка")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.AddMessage(userID, msg)
	}
}

func BenchmarkSessionManager_GetHistory(b *testing.B) {
	sm := NewSessionManager(12, 1*time.Hour)
	userID := int64(98765)
	for i := 0; i < 12; i++ {
		sm.AddUserMessage(userID, "Вопрос")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sm.GetHistory(userID)
	}
}

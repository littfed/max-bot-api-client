package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/littfed/max-bot-api-client/internal/platform"
	"github.com/littfed/max-bot-api-client/pkg/store"
)

var (
	// ErrForbidden is returned when a user does not have admin/superadmin privileges.
	ErrForbidden = errors.New("forbidden: insufficient permissions")
	// ErrNotFound is returned when a requested resource is not found.
	ErrNotFound = errors.New("resource not found")
)

// Store defines the storage operations required by the Admin service.
type Store interface {
	GetUser(ctx context.Context, id int64) (*store.User, error)
	GetUserIdentity(ctx context.Context, userID int64, platform string) (*store.UserIdentity, error)
	GetLeads(ctx context.Context, status string) ([]store.Lead, error)
	UpdateLeadStatus(ctx context.Context, leadID int64, status string) error
	GetSupportTickets(ctx context.Context, status string) ([]store.SupportTicket, error)
	GetSupportTicket(ctx context.Context, ticketID int64) (*store.SupportTicket, error)
	UpdateSupportTicketStatus(ctx context.Context, ticketID int64, status string) error
	SaveSupportMessage(ctx context.Context, msg *store.SupportMessage) error
	SaveAdminAuditLog(ctx context.Context, log *store.AdminAuditLog) error
	GetAdminAuditLogs(ctx context.Context, limit int) ([]store.AdminAuditLog, error)
}

// Guard defines the maintenance guard contract for updating mode states.
type Guard interface {
	SetModeActive(ctx context.Context, modeID string, isActive bool, maintMsg string) error
}

// Service provides hidden admin management functionality including maintenance toggles,
// lead management, ticket support reply handling, and audit logging with RBAC enforcement.
type Service struct {
	store   Store
	guard   Guard
	adapter platform.PlatformAdapter
}

// NewService creates a new Admin Service.
func NewService(store Store, guard Guard, adapter platform.PlatformAdapter) *Service {
	return &Service{
		store:   store,
		guard:   guard,
		adapter: adapter,
	}
}

// checkAdmin verifies that the user with adminID exists and possesses admin or superadmin privileges.
func (s *Service) checkAdmin(ctx context.Context, adminID int64) error {
	if s.store == nil {
		return ErrForbidden
	}
	user, err := s.store.GetUser(ctx, adminID)
	if err != nil || user == nil {
		return ErrForbidden
	}
	if user.Role != "admin" && user.Role != "superadmin" {
		return ErrForbidden
	}
	return nil
}

// ToggleMode switches a mode's active state, updates maintenance message, invalidates cache,
// and records an audit log entry.
func (s *Service) ToggleMode(ctx context.Context, adminID int64, modeID string, isActive bool, maintMsg string, ip string) error {
	if err := s.checkAdmin(ctx, adminID); err != nil {
		return err
	}

	if s.guard != nil {
		if err := s.guard.SetModeActive(ctx, modeID, isActive, maintMsg); err != nil {
			return fmt.Errorf("failed to toggle mode %s: %w", modeID, err)
		}
	}

	diff, _ := json.Marshal(map[string]any{
		"is_active":           isActive,
		"maintenance_message": maintMsg,
	})

	audit := &store.AdminAuditLog{
		AdminID:     adminID,
		Action:      "toggle_maintenance",
		TargetMode:  modeID,
		PayloadDiff: diff,
		IP:          ip,
		CreatedAt:   time.Now(),
	}
	_ = s.store.SaveAdminAuditLog(ctx, audit)

	return nil
}

// ListLeads returns leads optionally filtered by status.
func (s *Service) ListLeads(ctx context.Context, adminID int64, status string) ([]store.Lead, error) {
	if err := s.checkAdmin(ctx, adminID); err != nil {
		return nil, err
	}
	return s.store.GetLeads(ctx, status)
}

// UpdateLeadStatus modifies the status of an existing lead and logs the action.
func (s *Service) UpdateLeadStatus(ctx context.Context, adminID int64, leadID int64, status string, ip string) error {
	if err := s.checkAdmin(ctx, adminID); err != nil {
		return err
	}

	if err := s.store.UpdateLeadStatus(ctx, leadID, status); err != nil {
		return fmt.Errorf("failed to update lead status: %w", err)
	}

	diff, _ := json.Marshal(map[string]any{
		"lead_id": leadID,
		"status":  status,
	})

	audit := &store.AdminAuditLog{
		AdminID:     adminID,
		Action:      "update_lead_status",
		TargetMode:  "leads",
		PayloadDiff: diff,
		IP:          ip,
		CreatedAt:   time.Now(),
	}
	_ = s.store.SaveAdminAuditLog(ctx, audit)

	return nil
}

// ListTickets returns support tickets optionally filtered by status.
func (s *Service) ListTickets(ctx context.Context, adminID int64, status string) ([]store.SupportTicket, error) {
	if err := s.checkAdmin(ctx, adminID); err != nil {
		return nil, err
	}
	return s.store.GetSupportTickets(ctx, status)
}

// ReplyTicket records an operator's reply to a support ticket, dispatches the message to the user
// via PlatformAdapter, marks the ticket as answered, and records an audit log.
func (s *Service) ReplyTicket(ctx context.Context, adminID int64, ticketID int64, messageText string, ip string) error {
	if err := s.checkAdmin(ctx, adminID); err != nil {
		return err
	}

	ticket, err := s.store.GetSupportTicket(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("failed to fetch ticket %d: %w", ticketID, err)
	}
	if ticket == nil {
		return ErrNotFound
	}

	// 1. Record operator reply message in database
	msg := &store.SupportMessage{
		TicketID:    ticketID,
		SenderType:  "operator",
		MessageText: messageText,
		CreatedAt:   time.Now(),
	}
	if err := s.store.SaveSupportMessage(ctx, msg); err != nil {
		return fmt.Errorf("failed to save support message: %w", err)
	}

	// 2. Deliver message to recipient via PlatformAdapter
	if s.adapter != nil {
		recipientID := strconv.FormatInt(ticket.UserID, 10)
		if ident, err := s.store.GetUserIdentity(ctx, ticket.UserID, ticket.Platform); err == nil && ident != nil && ident.PlatformUserID != "" {
			recipientID = ident.PlatformUserID
		}

		out := &platform.OutMessage{
			Text: messageText,
		}
		if err := s.adapter.SendMessage(ctx, recipientID, out); err != nil {
			return fmt.Errorf("failed to deliver message via platform adapter: %w", err)
		}
	}

	// 3. Mark ticket as answered
	if err := s.store.UpdateSupportTicketStatus(ctx, ticketID, "answered"); err != nil {
		return fmt.Errorf("failed to update ticket status: %w", err)
	}

	// 4. Record audit log
	diff, _ := json.Marshal(map[string]any{
		"ticket_id":    ticketID,
		"message_text": messageText,
	})

	audit := &store.AdminAuditLog{
		AdminID:     adminID,
		Action:      "reply_ticket",
		TargetMode:  "support",
		PayloadDiff: diff,
		IP:          ip,
		CreatedAt:   time.Now(),
	}
	_ = s.store.SaveAdminAuditLog(ctx, audit)

	return nil
}

// GetAuditLogs returns the most recent admin audit logs up to the specified limit.
func (s *Service) GetAuditLogs(ctx context.Context, adminID int64, limit int) ([]store.AdminAuditLog, error) {
	if err := s.checkAdmin(ctx, adminID); err != nil {
		return nil, err
	}
	return s.store.GetAdminAuditLogs(ctx, limit)
}

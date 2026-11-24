package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/littfed/max-bot-api-client/internal/admin"
	"github.com/littfed/max-bot-api-client/internal/core"
	"github.com/littfed/max-bot-api-client/internal/modules/chat"
	"github.com/littfed/max-bot-api-client/internal/modules/reception"
	"github.com/littfed/max-bot-api-client/internal/modules/shop"
	"github.com/littfed/max-bot-api-client/internal/modules/support"
	"github.com/littfed/max-bot-api-client/internal/platform"
	"github.com/littfed/max-bot-api-client/pkg/store"
	"github.com/littfed/max-bot-api-client/pkg/webapp"
)

type contextKey string

const (
	userContextKey     = contextKey("auth_user")
	platformContextKey = contextKey("platform")
)

// RouterConfig holds all service, storage, and module dependencies for the API Router.
type RouterConfig struct {
	Store           *store.SQLStore
	Guard           *core.MaintenanceGuard
	FSM             *core.CompositeFSM
	Orchestrator    *core.Orchestrator
	AdminService    *admin.Service
	ShopModule      *shop.Module
	ReceptionModule *reception.Module
	SupportModule   *support.Module
	ChatModule      *chat.Module
	BotToken        string
	JWTSecret       string
	PlatformAdapter platform.PlatformAdapter
	AllowedOrigins  []string
	AllowDevHeaders bool
}

// UserFromContext retrieves the authenticated store.User from the request context.
func UserFromContext(ctx context.Context) *store.User {
	if u, ok := ctx.Value(userContextKey).(*store.User); ok {
		return u
	}
	return nil
}

// PlatformFromContext retrieves the client platform (max, telegram, web) from context.
func PlatformFromContext(ctx context.Context) string {
	if p, ok := ctx.Value(platformContextKey).(string); ok && p != "" {
		return p
	}
	return "max"
}

// JWTClaims represents claims encoded within the JWT auth token.
type JWTClaims struct {
	UserID    int64  `json:"sub"`
	Role      string `json:"role"`
	Platform  string `json:"platform"`
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
}

// GenerateJWT creates a standard HS256 signed JWT for an authenticated user.
func GenerateJWT(claims JWTClaims, secret string) (string, error) {
	if secret == "" {
		secret = "default_bothub_jwt_secret_key"
	}

	headerJSON, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", fmt.Errorf("jwt: failed to marshal header: %w", err)
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("jwt: failed to marshal claims: %w", err)
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := headerB64 + "." + claimsB64

	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(signingInput))
	signatureB64 := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return signingInput + "." + signatureB64, nil
}

// ValidateJWT verifies the cryptographic signature and expiration of an HS256 JWT.
func ValidateJWT(tokenStr string, secret string) (*JWTClaims, error) {
	if secret == "" {
		secret = "default_bothub_jwt_secret_key"
	}

	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, errors.New("jwt: invalid token format")
	}

	signingInput := parts[0] + "." + parts[1]
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(signingInput))
	expectedSig := h.Sum(nil)

	receivedSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("jwt: invalid signature encoding")
	}

	if subtle.ConstantTimeCompare(receivedSig, expectedSig) != 1 {
		return nil, errors.New("jwt: signature mismatch")
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("jwt: invalid claims encoding")
	}

	var claims JWTClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, fmt.Errorf("jwt: invalid claims json: %w", err)
	}

	if claims.ExpiresAt > 0 && time.Now().Unix() > claims.ExpiresAt {
		return nil, errors.New("jwt: token has expired")
	}

	return &claims, nil
}

// writeJSON serializes data as JSON with appropriate headers and status code.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			slog.Error("failed to encode json response", "error", err)
		}
	}
}

// writeError serializes an error message in standard JSON error envelope.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"error":   http.StatusText(status),
		"message": message,
	})
}

// NewRouter constructs and mounts all API routes under /api/v1/.
func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	hubH := newHubHandler(cfg)
	modH := newModulesHandler(cfg)
	admH := newAdminHandler(cfg)

	// Hub Endpoints
	mux.HandleFunc("GET /api/v1/bootstrap", hubH.handleBootstrap)
	mux.HandleFunc("POST /api/v1/user/mode", hubH.handleUserMode)
	mux.HandleFunc("GET /api/v1/mascot/tap", hubH.handleMascotTap)
	mux.HandleFunc("POST /api/v1/lead", hubH.handleLead)

	// Shop Module Endpoints
	mux.HandleFunc("GET /api/v1/shop/catalog", modH.handleShopCatalog)
	mux.HandleFunc("POST /api/v1/shop/orders", modH.handleShopCreateOrder)
	mux.HandleFunc("POST /api/v1/shop/orders/{id}/simulate", modH.handleShopSimulate)

	// Reception Module Endpoints
	mux.HandleFunc("GET /api/v1/reception/slots", modH.handleReceptionSlots)
	mux.HandleFunc("POST /api/v1/reception/book", modH.handleReceptionBook)
	mux.HandleFunc("GET /api/v1/reception/events", modH.handleReceptionEvents)
	mux.HandleFunc("POST /api/v1/reception/events/{id}/register", modH.handleReceptionRegisterEvent)
	mux.HandleFunc("POST /api/v1/reception/onboarding", modH.handleReceptionOnboarding)

	// Support Module Endpoints
	mux.HandleFunc("POST /api/v1/support/ask", modH.handleSupportAsk)
	mux.HandleFunc("POST /api/v1/support/tickets", modH.handleSupportCreateTicket)
	mux.HandleFunc("GET /api/v1/support/tickets", modH.handleSupportGetTickets)

	// Chat Module Endpoints
	mux.HandleFunc("GET /api/v1/chat/config", modH.handleChatConfig)
	mux.HandleFunc("GET /api/v1/chat/session", modH.handleChatSession)
	mux.HandleFunc("POST /api/v1/chat/settings", modH.handleChatSettings)
	mux.HandleFunc("POST /api/v1/chat/message", modH.handleChatMessage)
	mux.HandleFunc("POST /api/v1/chat/clear", modH.handleChatClear)

	// Admin Endpoints (protected by requireAdmin middleware)
	mux.HandleFunc("GET /api/v1/admin/modes", requireAdmin(admH.handleAdminModes))
	mux.HandleFunc("POST /api/v1/admin/modes/{id}/toggle", requireAdmin(admH.handleAdminToggleMode))
	mux.HandleFunc("GET /api/v1/admin/leads", requireAdmin(admH.handleAdminLeads))
	mux.HandleFunc("POST /api/v1/admin/leads/{id}/status", requireAdmin(admH.handleAdminLeadStatus))
	mux.HandleFunc("GET /api/v1/admin/tickets", requireAdmin(admH.handleAdminTickets))
	mux.HandleFunc("POST /api/v1/admin/tickets/{id}/reply", requireAdmin(admH.handleAdminTicketReply))
	mux.HandleFunc("GET /api/v1/admin/audit-logs", requireAdmin(admH.handleAdminAuditLogs))

	// Middleware chaining: CORS -> Auth -> Handler
	return corsMiddleware(cfg, authMiddleware(cfg, mux))
}

// corsMiddleware handles preflight OPTIONS requests and sets CORS headers.
func corsMiddleware(cfg RouterConfig, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := false
		if len(cfg.AllowedOrigins) > 0 {
			for _, o := range cfg.AllowedOrigins {
				if o == origin || o == "*" {
					allowed = true
					break
				}
			}
		} else {
			allowed = true
		}

		if allowed || origin == "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Init-Data, X-Platform, X-Dev-User-ID, X-Test-User-ID")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// authMiddleware validates user credentials via X-Init-Data, JWT Authorization header, or dev headers.
func authMiddleware(cfg RouterConfig, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		platformName := r.Header.Get("X-Platform")
		if platformName == "" {
			platformName = "max"
		}

		var authedUser *store.User

		// 1. Try Dev / Test header (X-Dev-User-ID or X-Test-User-ID) only if AllowDevHeaders is explicitly enabled
		if cfg.AllowDevHeaders {
			devUserIDStr := r.Header.Get("X-Dev-User-ID")
			if devUserIDStr == "" {
				devUserIDStr = r.Header.Get("X-Test-User-ID")
			}
			if devUserIDStr != "" {
				if id, err := strconv.ParseInt(devUserIDStr, 10, 64); err == nil && cfg.Store != nil {
					if u, err := cfg.Store.GetUser(ctx, id); err == nil && u != nil {
						authedUser = u
					}
				}
			}
		}

		// 2. Try X-Init-Data header or query param
		if authedUser == nil {
			initDataStr := r.Header.Get("X-Init-Data")
			if initDataStr == "" {
				initDataStr = r.URL.Query().Get("init_data")
			}
			if initDataStr != "" {
				initData, err := webapp.ValidateInitData(initDataStr, cfg.BotToken)
				if err != nil {
					writeError(w, http.StatusUnauthorized, fmt.Sprintf("invalid initData: %v", err))
					return
				}
				if initData.User != nil && cfg.Store != nil {
					pUserID := strconv.FormatInt(initData.User.ID, 10)
					u, err := cfg.Store.ResolveUserIdentity(ctx, platformName, pUserID, initData.User.Username, initData.User.FirstName, "")
					if err != nil {
						writeError(w, http.StatusInternalServerError, "failed to resolve user identity")
						return
					}
					authedUser = u
				}
			}
		}

		// 3. Try Authorization: Bearer <token>
		if authedUser == nil {
			authHdr := r.Header.Get("Authorization")
			if strings.HasPrefix(authHdr, "Bearer ") {
				tokenStr := strings.TrimPrefix(authHdr, "Bearer ")
				claims, err := ValidateJWT(tokenStr, cfg.JWTSecret)
				if err == nil && claims != nil && cfg.Store != nil {
					if u, err := cfg.Store.GetUser(ctx, claims.UserID); err == nil && u != nil {
						authedUser = u
						if r.Header.Get("X-Platform") == "" && claims.Platform != "" {
							platformName = claims.Platform
						}
					}
				}
			}
		}

		if authedUser == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized: missing or invalid credentials")
			return
		}

		ctx = context.WithValue(ctx, userContextKey, authedUser)
		ctx = context.WithValue(ctx, platformContextKey, platformName)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireAdmin enforces RBAC for administrator-only routes.
func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil || (user.Role != "admin" && user.Role != "superadmin") {
			writeError(w, http.StatusForbidden, "forbidden: admin privileges required")
			return
		}
		next(w, r)
	}
}

package bot

import (
	"regexp"
	"strings"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// HandlerFunc is the signature for handling an incoming bot update.
type HandlerFunc func(c *Context) error

// MiddlewareFunc wraps a HandlerFunc with cross-cutting concerns (logging, recovery, etc.).
type MiddlewareFunc func(next HandlerFunc) HandlerFunc

type matcherFunc func(c *Context) bool

type route struct {
	matches matcherFunc
	handler HandlerFunc
}

// Router dispatches incoming events to matching handlers.
type Router struct {
	routes      []route
	middlewares []MiddlewareFunc
}

// NewRouter creates a new empty Router.
func NewRouter() *Router {
	return &Router{
		routes:      make([]route, 0),
		middlewares: make([]MiddlewareFunc, 0),
	}
}

// Use adds one or more middlewares to the router pipeline.
func (r *Router) Use(mw ...MiddlewareFunc) {
	r.middlewares = append(r.middlewares, mw...)
}

func (r *Router) addRoute(matcher matcherFunc, handler HandlerFunc) {
	r.routes = append(r.routes, route{
		matches: matcher,
		handler: handler,
	})
}

// OnCommand registers a handler for a specific command (e.g. "ping", "/help").
func (r *Router) OnCommand(command string, handler HandlerFunc) {
	cmdClean := strings.ToLower(strings.TrimPrefix(command, "/"))
	r.addRoute(func(c *Context) bool {
		cmd, _ := c.Command()
		return cmd == cmdClean
	}, handler)
}

// OnText registers a handler for exact text matching (case-insensitive).
func (r *Router) OnText(text string, handler HandlerFunc) {
	target := strings.ToLower(strings.TrimSpace(text))
	r.addRoute(func(c *Context) bool {
		return strings.ToLower(strings.TrimSpace(c.Text())) == target
	}, handler)
}

// OnTextRegex registers a handler matching message text against a regular expression.
func (r *Router) OnTextRegex(re *regexp.Regexp, handler HandlerFunc) {
	r.addRoute(func(c *Context) bool {
		return re.MatchString(c.Text())
	}, handler)
}

// OnStart registers a handler for bot launch (both `bot_started` updates and `/start` commands).
func (r *Router) OnStart(handler HandlerFunc) {
	r.addRoute(func(c *Context) bool {
		if c.Update != nil && c.Update.UpdateType == maxbot.UpdateTypeBotStarted {
			return true
		}
		cmd, _ := c.Command()
		return cmd == "start"
	}, handler)
}

// OnMessage registers a handler for any incoming message creation event.
func (r *Router) OnMessage(handler HandlerFunc) {
	r.addRoute(func(c *Context) bool {
		return c.Update != nil && c.Update.UpdateType == maxbot.UpdateTypeMessageCreated
	}, handler)
}

// OnCallback registers a handler for any callback query event.
func (r *Router) OnCallback(handler HandlerFunc) {
	r.addRoute(func(c *Context) bool {
		return c.Callback() != nil
	}, handler)
}

// OnCallbackPayload registers a handler for a callback query matching an exact payload string.
func (r *Router) OnCallbackPayload(payload string, handler HandlerFunc) {
	r.addRoute(func(c *Context) bool {
		cb := c.Callback()
		return cb != nil && cb.Payload == payload
	}, handler)
}

// OnCallbackPrefix registers a handler for a callback query whose payload starts with prefix.
func (r *Router) OnCallbackPrefix(prefix string, handler HandlerFunc) {
	r.addRoute(func(c *Context) bool {
		cb := c.Callback()
		return cb != nil && strings.HasPrefix(cb.Payload, prefix)
	}, handler)
}

// OnAction registers a handler matching either a plain callback payload or an encoded tracked action (trk:<action>|...).
func (r *Router) OnAction(action string, handler HandlerFunc) {
	r.addRoute(func(c *Context) bool {
		cb := c.Callback()
		if cb == nil {
			return false
		}
		if cb.Payload == action {
			return true
		}
		if strings.HasPrefix(cb.Payload, "trk:") {
			parts := strings.Split(strings.TrimPrefix(cb.Payload, "trk:"), "|")
			return len(parts) > 0 && parts[0] == action
		}
		return false
	}, handler)
}

// OnState registers a handler executed only when the user's active FSM dialog state matches state and the update is not a callback query.
func (r *Router) OnState(state string, handler HandlerFunc) {
	r.addRoute(func(c *Context) bool {
		if c.Callback() != nil {
			return false
		}
		val, ok := c.Get("fsm_context")
		if !ok || val == nil {
			return false
		}
		if sg, ok := val.(interface{ GetState() (string, error) }); ok {
			st, err := sg.GetState()
			return err == nil && st == state
		}
		return false
	}, handler)
}

// OnStateCallback registers a handler executed when in a specific state and receiving a matching callback payload.
func (r *Router) OnStateCallback(state string, payload string, handler HandlerFunc) {
	r.addRoute(func(c *Context) bool {
		cb := c.Callback()
		if cb == nil || cb.Payload != payload {
			return false
		}
		val, ok := c.Get("fsm_context")
		if !ok || val == nil {
			return false
		}
		if sg, ok := val.(interface{ GetState() (string, error) }); ok {
			st, err := sg.GetState()
			return err == nil && st == state
		}
		return false
	}, handler)
}

// OnBotAdded registers a handler invoked when the bot is added to a group chat or channel.
func (r *Router) OnBotAdded(handler HandlerFunc) {
	r.addRoute(func(c *Context) bool {
		return c.Update != nil && c.Update.UpdateType == maxbot.UpdateTypeBotAdded
	}, handler)
}

// OnBotRemoved registers a handler invoked when the bot is removed from a group chat or channel.
func (r *Router) OnBotRemoved(handler HandlerFunc) {
	r.addRoute(func(c *Context) bool {
		return c.Update != nil && c.Update.UpdateType == maxbot.UpdateTypeBotRemoved
	}, handler)
}

// OnAny registers a fallback handler matching any update.
func (r *Router) OnAny(handler HandlerFunc) {
	r.addRoute(func(c *Context) bool {
		return true
	}, handler)
}

// Handle executes the first matching route handler wrapped with all registered middlewares.
func (r *Router) Handle(c *Context) error {
	dispatch := func(ctx *Context) error {
		for _, rt := range r.routes {
			if rt.matches(ctx) {
				return rt.handler(ctx)
			}
		}
		return nil
	}

	handler := dispatch
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		handler = r.middlewares[i](handler)
	}
	return handler(c)
}

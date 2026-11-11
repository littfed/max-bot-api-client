package webapp

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

var (
	// ErrURLInWebAppButton is returned when a URL is mistakenly passed to an open_app button instead of a bot slug.
	ErrURLInWebAppButton = errors.New("webapp: open_app buttons accept only registered app usernames/slugs (e.g. 'my_bot' or 'id123_bot'), not arbitrary URLs. For external web links, use maxbot.NewLinkButton()")
)

// OpenAppButton creates an interactive open_app button for launching a Mini App inside MAX.
// In MAX Bot API, the webApp field must refer to a registered bot or app username (not an HTTP URL).
func OpenAppButton(text string, appSlug string, startParam ...string) (maxbot.Button, error) {
	slugClean := strings.TrimSpace(appSlug)
	if slugClean == "" {
		return maxbot.Button{}, errors.New("webapp: appSlug cannot be empty")
	}

	if strings.HasPrefix(slugClean, "http://") || strings.HasPrefix(slugClean, "https://") {
		return maxbot.Button{}, ErrURLInWebAppButton
	}

	slugWithParam := slugClean
	if len(startParam) > 0 && startParam[0] != "" {
		slugWithParam = fmt.Sprintf("%s?start=%s", slugClean, url.QueryEscape(startParam[0]))
	}

	return maxbot.Button{
		Type:   maxbot.ButtonTypeOpenApp,
		Text:   text,
		WebApp: slugWithParam,
	}, nil
}

// GenerateReturnLink builds a standard deep link enabling the user to return from a webview back to the bot chat with a payload token.
func GenerateReturnLink(botUsername string, startParam string) string {
	cleanUsername := strings.TrimPrefix(strings.TrimSpace(botUsername), "@")
	if startParam == "" {
		return fmt.Sprintf("https://max.ru/%s", cleanUsername)
	}
	return fmt.Sprintf("https://max.ru/%s?start=%s", cleanUsername, url.QueryEscape(startParam))
}

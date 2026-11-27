package posts

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// PlatformSpec describes formatting, length, and visual constraints for a target social platform.
type PlatformSpec struct {
	ID              string `json:"id"`
	DisplayName     string `json:"display_name"`
	MaxChars        int    `json:"max_chars"`
	CaptionMaxChars int    `json:"caption_max_chars"`
	MarkupStyle     string `json:"markup_style"`
	HashtagHint     string `json:"hashtag_hint"`
	AspectRatio     string `json:"aspect_ratio"`
	StylePrompt     string `json:"style_prompt"`
}

var defaultPlatforms = []string{"max", "telegram"}

var platformSpecs = []PlatformSpec{
	{
		ID:              "max",
		DisplayName:     "MAX",
		MaxChars:        4000,
		CaptionMaxChars: 4000,
		MarkupStyle:     "markdown",
		HashtagHint:     "0–3 тега по смыслу",
		AspectRatio:     "16:9",
		StylePrompt:     "Структурированный пост для канала в MAX: чёткие абзацы, списки, выделение ключевых мыслей, без лишней воды.",
	},
	{
		ID:              "telegram",
		DisplayName:     "Telegram",
		MaxChars:        4096,
		CaptionMaxChars: 1024,
		MarkupStyle:     "markdown",
		HashtagHint:     "1–3 релевантных тега в конце",
		AspectRatio:     "16:9",
		StylePrompt:     "Живой пост для Telegram: сильный первый абзац, короткие строки, читаемый ритм и аккуратные акценты.",
	},
	{
		ID:              "vk",
		DisplayName:     "ВКонтакте",
		MaxChars:        15000,
		CaptionMaxChars: 15000,
		MarkupStyle:     "plain",
		HashtagHint:     "3–5 тематических тегов",
		AspectRatio:     "4:5",
		StylePrompt:     "Развёрнутый пост для ВКонтакте: вовлекающее вступление, полезная основная часть, понятный CTA, без Markdown-разметки.",
	},
	{
		ID:              "instagram",
		DisplayName:     "Instagram",
		MaxChars:        2200,
		CaptionMaxChars: 2200,
		MarkupStyle:     "plain",
		HashtagHint:     "5–10 целевых тегов в отдельном блоке",
		AspectRatio:     "4:5",
		StylePrompt:     "Лаконичный пост для Instagram: цепляющая первая строка, короткие абзацы, живой тон и явный призыв к действию.",
	},
}

var platformRegistry = func() map[string]PlatformSpec {
	m := make(map[string]PlatformSpec, len(platformSpecs))
	for _, spec := range platformSpecs {
		m[spec.ID] = spec
	}
	return m
}()

// GetPlatformSpec returns the declarative specification for a platform ID.
func GetPlatformSpec(id string) (PlatformSpec, bool) {
	spec, ok := platformRegistry[strings.ToLower(strings.TrimSpace(id))]
	return spec, ok
}

// AllPlatformSpecs returns all registered platform specifications in canonical order.
func AllPlatformSpecs() []PlatformSpec {
	out := make([]PlatformSpec, len(platformSpecs))
	copy(out, platformSpecs)
	return out
}

// NormalizePlatforms filters unknown IDs, normalizes case, deduplicates preserving order,
// and falls back to the default platforms when the resulting list is empty.
func NormalizePlatforms(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.ToLower(strings.TrimSpace(raw))
		if _, ok := platformRegistry[id]; !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		fallback := make([]string, len(defaultPlatforms))
		copy(fallback, defaultPlatforms)
		return fallback
	}
	return out
}

// ValidateVariant checks a generated PostVariant against platform character limits and forbidden words.
func ValidateVariant(spec PlatformSpec, v *PostVariant, forbiddenWords string) []string {
	if v == nil {
		return nil
	}

	var warnings []string

	if spec.MaxChars > 0 {
		runeLen := utf8.RuneCountInString(v.Text)
		if runeLen > spec.MaxChars {
			warnings = append(warnings, fmt.Sprintf(
				"превышен лимит длины для %s: %d символов при максимуме %d",
				spec.ID, runeLen, spec.MaxChars,
			))
		}
	}

	if strings.TrimSpace(forbiddenWords) != "" {
		searchable := strings.ToLower(v.Text + "\n" + v.CTA + "\n" + strings.Join(v.Hashtags, " "))
		parts := strings.FieldsFunc(forbiddenWords, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n'
		})
		for _, rawWord := range parts {
			word := strings.TrimSpace(rawWord)
			if word == "" {
				continue
			}
			if strings.Contains(searchable, strings.ToLower(word)) {
				warnings = append(warnings, fmt.Sprintf("обнаружено запрещённое слово или фраза: %q", word))
			}
		}
	}

	return warnings
}

package posts

import (
	"reflect"
	"strings"
	"testing"
)

func TestPlatformSpecAndValidation(t *testing.T) {
	t.Run("GetPlatformSpec instagram and core platforms", func(t *testing.T) {
		ig, ok := GetPlatformSpec("instagram")
		if !ok {
			t.Fatalf("expected GetPlatformSpec(%q) ok == true, got false", "instagram")
		}
		if ig.MaxChars != 2200 {
			t.Errorf("expected instagram MaxChars == 2200, got %d", ig.MaxChars)
		}
		if ig.AspectRatio != "4:5" {
			t.Errorf("expected instagram AspectRatio == %q, got %q", "4:5", ig.AspectRatio)
		}

		for _, id := range []string{"vk", "telegram", "max"} {
			spec, exists := GetPlatformSpec(id)
			if !exists {
				t.Errorf("expected GetPlatformSpec(%q) ok == true, got false", id)
			}
			if spec.ID != id {
				t.Errorf("expected spec.ID == %q, got %q", id, spec.ID)
			}
		}

		if _, exists := GetPlatformSpec("unknown"); exists {
			t.Errorf("expected GetPlatformSpec(%q) ok == false, got true", "unknown")
		}

		all := AllPlatformSpecs()
		if len(all) != 4 {
			t.Errorf("expected AllPlatformSpecs() to return 4 specs, got %d", len(all))
		}
	})

	t.Run("NormalizePlatforms", func(t *testing.T) {
		got := NormalizePlatforms([]string{"UNKNOWN", "max", "VK", "max"})
		want := []string{"max", "vk"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("NormalizePlatforms(mixed) = %v, want %v", got, want)
		}

		gotDefault := NormalizePlatforms(nil)
		wantDefault := []string{"max", "telegram"}
		if !reflect.DeepEqual(gotDefault, wantDefault) {
			t.Errorf("NormalizePlatforms(nil) = %v, want %v", gotDefault, wantDefault)
		}
	})

	t.Run("ValidateVariant forbidden words and rune overflow", func(t *testing.T) {
		spec := PlatformSpec{
			ID:       "telegram",
			MaxChars: 20,
		}

		longCyrillic := strings.Repeat("а", 21)
		warnings := ValidateVariant(spec, &PostVariant{
			Platform: "telegram",
			Text:     "Это УСПЕШНЫЙ УСПЕХ и " + longCyrillic,
		}, "успешный успех, кринж")

		if len(warnings) < 2 {
			t.Fatalf("expected at least 2 warnings (forbidden word + length overflow), got %d: %v", len(warnings), warnings)
		}

		hasForbidden := false
		hasOverflow := false
		for _, w := range warnings {
			lw := strings.ToLower(w)
			if strings.Contains(lw, "успешный успех") {
				hasForbidden = true
			}
			if strings.Contains(lw, "20") || strings.Contains(lw, "21") || strings.Contains(lw, "длин") || strings.Contains(lw, "лимит") {
				hasOverflow = true
			}
		}
		if !hasForbidden {
			t.Errorf("expected forbidden word warning for %q, got %v", "успешный успех", warnings)
		}
		if !hasOverflow {
			t.Errorf("expected length overflow warning, got %v", warnings)
		}

		cleanWarnings := ValidateVariant(spec, &PostVariant{
			Platform: "telegram",
			Text:     "Короткий пост",
		}, "успешный успех, кринж")
		if len(cleanWarnings) != 0 {
			t.Errorf("expected 0 warnings for clean short variant, got %v", cleanWarnings)
		}
	})
}

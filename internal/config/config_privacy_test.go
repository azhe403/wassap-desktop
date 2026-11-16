package config_test

import (
	"path/filepath"
	"sync"
	"testing"

	"wassap-desktop/internal/config"
)

func TestGetPrivacyBlurSettingsUnwritten(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))

	got, err := store.GetPrivacyBlurSettings()
	if err != nil {
		t.Fatalf("GetPrivacyBlurSettings() unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for unwritten section, got %+v", got)
	}

	def := config.DefaultPrivacyBlurConfig()
	if def.Enabled {
		t.Error("default master switch must be off")
	}
	if def.PrivacyMode != "blur" {
		t.Errorf("default mode must be blur, got %q", def.PrivacyMode)
	}
	if def.BlurAmount != 3 || def.InputOpacity != 30 || def.UnblurLastNCount != 3 {
		t.Errorf("unexpected defaults: %+v", def)
	}
	if !def.BlurAvatars || !def.BlurNames || !def.BlurPreviews || !def.BlurTextChats ||
		!def.BlurStickers || !def.BlurMediaPreview || !def.BlurMediaGallery || !def.BlurInput {
		t.Error("all per-target defaults must be on")
	}
	if !def.Animation || def.AnimationDuration != 0.25 {
		t.Error("animation defaults must be on at 0.25s")
	}
}

func TestPrivacyBlurRoundTrip(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))

	want := config.DefaultPrivacyBlurConfig()
	want.Enabled = true
	want.PrivacyMode = "redacted"
	want.BlurAmount = 7
	want.Animation = false
	want.AnimationDuration = 0.5
	want.InputOpacity = 80
	want.UnblurLastN = true
	want.UnblurLastNCount = 9
	want.BlurStickers = false

	if err := store.SetPrivacyBlurSettings(want); err != nil {
		t.Fatalf("SetPrivacyBlurSettings() failed: %v", err)
	}
	got, err := store.GetPrivacyBlurSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected settings after Set, got nil")
	}
	if *got != want {
		t.Errorf("round trip mismatch:\nwant %+v\ngot  %+v", want, *got)
	}
}

func TestSetPrivacyBlurClamps(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))

	bad := config.DefaultPrivacyBlurConfig()
	bad.BlurAmount = -3
	bad.InputOpacity = 250
	bad.UnblurLastNCount = 42
	bad.PrivacyMode = "fancy"
	bad.AnimationDuration = 0

	if err := store.SetPrivacyBlurSettings(bad); err != nil {
		t.Fatalf("SetPrivacyBlurSettings() failed: %v", err)
	}
	got, err := store.GetPrivacyBlurSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.BlurAmount != 1 {
		t.Errorf("blurAmount -3 must clamp to 1, got %d", got.BlurAmount)
	}
	if got.InputOpacity != 100 {
		t.Errorf("inputOpacity 250 must clamp to 100, got %d", got.InputOpacity)
	}
	if got.UnblurLastNCount != 10 {
		t.Errorf("unblurLastNCount 42 must clamp to 10, got %d", got.UnblurLastNCount)
	}
	if got.PrivacyMode != "blur" {
		t.Errorf("invalid mode must fall back to blur, got %q", got.PrivacyMode)
	}
	if got.AnimationDuration != 0.05 {
		t.Errorf("animationDuration 0 must clamp to 0.05, got %v", got.AnimationDuration)
	}
}

func TestMergePrivacyBlurSettingsPartialKeepsRest(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))

	if err := store.SetPrivacyBlurSettings(config.DefaultPrivacyBlurConfig()); err != nil {
		t.Fatal(err)
	}
	merged, err := store.MergePrivacyBlurSettings(map[string]any{"blurAmount": float64(8)})
	if err != nil {
		t.Fatalf("MergePrivacyBlurSettings() failed: %v", err)
	}
	if merged.BlurAmount != 8 {
		t.Errorf("patched key not applied: %d", merged.BlurAmount)
	}
	if !merged.BlurAvatars || !merged.BlurNames || !merged.BlurPreviews {
		t.Error("untouched per-target keys must keep their values")
	}
	if merged.Enabled {
		t.Error("untouched enabled must stay false")
	}
}

func TestMergePrivacyBlurSettingsTypesAndClamps(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))

	merged, err := store.MergePrivacyBlurSettings(map[string]any{
		"enabled":           true,
		"privacyMode":       "redacted",
		"blurAmount":        float64(99),   // clamps to 10
		"inputOpacity":      float64(150),  // clamps to 100
		"unblurLastNCount":  float64(0),    // clamps to 1
		"animationDuration": float64(99),   // clamps to 2
		"unknownKey":        "ignored",     // dropped silently
		"blurAvatars":       "not-a-bool",  // wrong type: key ignored
		"blurNames":         false,         // applies
	})
	if err != nil {
		t.Fatalf("MergePrivacyBlurSettings() failed: %v", err)
	}
	if !merged.Enabled || merged.PrivacyMode != "redacted" {
		t.Errorf("bool/string keys not applied: %+v", merged)
	}
	if merged.BlurAmount != 10 || merged.InputOpacity != 100 || merged.UnblurLastNCount != 1 || merged.AnimationDuration != 2 {
		t.Errorf("numeric clamps wrong: %+v", merged)
	}
	if !merged.BlurAvatars {
		t.Error("wrong-typed blurAvatars must be ignored, not zeroed")
	}
	if merged.BlurNames {
		t.Error("blurNames=false must apply")
	}
}

func TestMergePrivacyBlurSettingsNilBaseStartsFromDefault(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))

	merged, err := store.MergePrivacyBlurSettings(map[string]any{"enabled": true})
	if err != nil {
		t.Fatal(err)
	}
	want := config.DefaultPrivacyBlurConfig()
	want.Enabled = true
	if *merged != want {
		t.Errorf("nil base must start from defaults with patch applied:\nwant %+v\ngot  %+v", want, *merged)
	}
}

func TestMergePrivacyBlurSettingsPreservesOtherConfigSections(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))

	if err := store.SetCloseToTray(true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.MergePrivacyBlurSettings(map[string]any{"enabled": true}); err != nil {
		t.Fatal(err)
	}
	tray, err := store.GetCloseToTray()
	if err != nil {
		t.Fatal(err)
	}
	if !tray {
		t.Error("privacy blur merge must not touch closeToTray")
	}
}

func TestMergePrivacyBlurSettingsConcurrent(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 25; j++ {
				_, err := store.MergePrivacyBlurSettings(map[string]any{
					"enabled":    j%2 == 0,
					"blurAmount": float64(n%10 + 1),
				})
				if err != nil {
					t.Errorf("merge failed: %v", err)
					return
				}
			}
		}(i)
	}
	wg.Wait()

	got, err := store.GetPrivacyBlurSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected settings after concurrent merges")
	}
	if got.BlurAmount < 1 || got.BlurAmount > 10 {
		t.Errorf("blurAmount torn by concurrent merges: %d", got.BlurAmount)
	}
}

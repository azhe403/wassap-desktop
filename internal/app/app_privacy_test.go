package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"wassap-desktop/internal/browser"
	"wassap-desktop/internal/config"
)

// fakeHost captures the privacy broadcast scripts instead of reaching a real
// WebView2 session.
type fakeHost struct {
	mu         sync.Mutex
	broadcasts []string
}

func (f *fakeHost) OpenProfile(id browser.ProfileID, probe browser.Probe) (browser.Session, error) {
	return nil, errors.New("fakeHost: not implemented")
}
func (f *fakeHost) CloseAll() error                       { return nil }
func (f *fakeHost) UpdateBounds(width, height int) error  { return nil }
func (f *fakeHost) SetVisible(visible bool) error         { return nil }
func (f *fakeHost) SetActiveSession(id browser.ProfileID) {}
func (f *fakeHost) EvalAll(script string) {
	f.mu.Lock()
	f.broadcasts = append(f.broadcasts, script)
	f.mu.Unlock()
}
func (f *fakeHost) SetPrivacyDelegate(prelude func() string, apply func() string, onWrite func(map[string]any)) {
}

func (f *fakeHost) SetAvatarDelegate(func(browser.ProfileID, string)) {}

func (f *fakeHost) SetSmartRAMSaver(bool) {}

func (f *fakeHost) last() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.broadcasts) == 0 {
		return ""
	}
	return f.broadcasts[len(f.broadcasts)-1]
}

func (f *fakeHost) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.broadcasts)
}

func newPrivacyTestApp(t *testing.T) (*App, *fakeHost, config.Store) {
	t.Helper()
	dirs := t.TempDir()
	cfgPath := filepath.Join(dirs, "config.json")
	store := config.NewStore(cfgPath)
	host := &fakeHost{}
	return &App{configStore: store, host: host}, host, store
}

func TestPrivacyBlurWriteBroadcastsMergedStoreState(t *testing.T) {
	app, host, store := newPrivacyTestApp(t)

	app.handlePrivacyBlurWrite(map[string]any{"enabled": true, "blurAmount": float64(6)})

	got, err := store.GetPrivacyBlurSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || !got.Enabled || got.BlurAmount != 6 {
		t.Fatalf("merge not persisted correctly: %+v", got)
	}
	last := host.last()
	if !strings.Contains(last, `"enabled":true`) || !strings.Contains(last, `"blurAmount":6`) {
		t.Fatalf("broadcast does not carry merged state: %s", last)
	}
}

func TestSetPrivacyBlurSettingsBroadcastsWholeObject(t *testing.T) {
	app, host, store := newPrivacyTestApp(t)

	cfg := config.DefaultPrivacyBlurConfig()
	cfg.Enabled = true
	cfg.BlurAmount = 9
	cfg.BlurNames = false
	if err := app.SetPrivacyBlurSettings(cfg); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetPrivacyBlurSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.BlurAmount != 9 || got.BlurNames {
		t.Fatalf("set not persisted: %+v", got)
	}
	last := host.last()
	for _, want := range []string{`"enabled":true`, `"blurAmount":9`, `"blurNames":false`} {
		if !strings.Contains(last, want) {
			t.Errorf("broadcast missing %s: %s", want, last)
		}
	}
}

// The broadcast after any write must carry the LATEST store state, not the
// state the writer computed. Concurrent writers interleave their store
// commits and their broadcasts, so without a shared lock the last broadcast
// may be built from a stale snapshot and sessions diverge (Review Focus #5).
func TestPrivacyBlurBroadcastsCarryLatestStoreStateUnderConcurrency(t *testing.T) {
	app, host, store := newPrivacyTestApp(t)

	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = app.SetPrivacyBlurSettings(func() config.PrivacyBlurConfig {
				c := config.DefaultPrivacyBlurConfig()
				c.Enabled = i%2 == 0
				c.BlurAmount = i%10 + 1
				return c
			}())
		}()
	}
	wg.Wait()

	got, err := store.GetPrivacyBlurSettings()
	if err != nil || got == nil {
		t.Fatalf("store read failed: %v", err)
	}
	last := host.last()
	wantEnabled := fmt.Sprintf(`"enabled":%t`, got.Enabled)
	wantAmount := fmt.Sprintf(`"blurAmount":%d`, got.BlurAmount)
	if !strings.Contains(last, wantEnabled) || !strings.Contains(last, wantAmount) {
		t.Fatalf("final broadcast does not match final store state:\nwant %s and %s\ngot  %s", wantEnabled, wantAmount, last)
	}
}

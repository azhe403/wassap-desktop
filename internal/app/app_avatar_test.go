package app

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"wassap-desktop/internal/appdirs"
	"wassap-desktop/internal/avatar"
)

func testAvatarURI(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 64, 64))); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func newAvatarTestApp(t *testing.T) *App {
	t.Helper()
	dirs := appdirs.Under(t.TempDir())
	return &App{dirs: dirs, avatarStore: avatar.NewStore(dirs.Data)}
}

func TestHandleAvatarUpdateStoresAndClears(t *testing.T) {
	app := newAvatarTestApp(t)
	uri := testAvatarURI(t)

	app.handleAvatarUpdate("account-a", uri)
	if got := app.GetAccountAvatar("account-a"); got != uri {
		t.Fatal("handleAvatarUpdate must store the photo")
	}
	path := filepath.Join(app.dirs.Data, "avatars", "account-a.png")
	if !fileExists(path) {
		t.Fatal("avatar file missing after update")
	}

	app.handleAvatarUpdate("account-a", "")
	if got := app.GetAccountAvatar("account-a"); got != "" {
		t.Fatal("empty data URI must clear the stored photo")
	}
	if fileExists(path) {
		t.Fatal("avatar file must be removed on clear")
	}
}

func TestHandleAvatarUpdateRejectsInvalidPayload(t *testing.T) {
	app := newAvatarTestApp(t)
	app.handleAvatarUpdate("account-a", "data:image/webp;base64,AA==")
	if got := app.GetAccountAvatar("account-a"); got != "" {
		t.Fatalf("invalid payload must not be stored, got %q", got)
	}
}

func TestGetAccountAvatarReturnsEmptyForMissing(t *testing.T) {
	app := newAvatarTestApp(t)
	if got := app.GetAccountAvatar("missing"); got != "" {
		t.Fatalf("GetAccountAvatar = %q, want empty", got)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

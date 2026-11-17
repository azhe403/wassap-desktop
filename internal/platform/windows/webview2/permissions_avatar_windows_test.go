//go:build windows

package webview2

import (
	"testing"
	"time"

	"github.com/wailsapp/go-webview2/pkg/edge"

	"wassap-desktop/internal/browser"
)

type avatarCall struct {
	profile browser.ProfileID
	uri     string
}

func TestAvatarMessagesReachDelegateWithSessionProfile(t *testing.T) {
	host := NewHost("t", "")
	calls := make(chan avatarCall, 4)
	host.SetAvatarDelegate(func(id browser.ProfileID, uri string) {
		calls <- avatarCall{id, uri}
	})
	s := &Session{host: host, profile: "a01"}
	c := &edge.Chromium{}
	wireNotificationClickMessage(s, c)

	c.MessageCallback(`{"kind":"avatar-update","account":"spoofed","values":{"dataUrl":"data:image/png;base64,AA=="}}`, nil, nil)
	c.MessageCallback(`{"kind":"avatar-clear"}`, nil, nil)

	first := awaitAvatarCall(t, calls)
	if first.profile != browser.ProfileID("a01") || first.uri != "data:image/png;base64,AA==" {
		t.Errorf("update call = %+v, want session profile a01 with the posted data URI", first)
	}
	second := awaitAvatarCall(t, calls)
	if second.profile != browser.ProfileID("a01") || second.uri != "" {
		t.Errorf("clear call = %+v, want session profile a01 with empty URI", second)
	}
}

func TestAvatarMessagesIgnoreMalformedJSON(t *testing.T) {
	host := NewHost("t", "")
	calls := make(chan avatarCall, 4)
	host.SetAvatarDelegate(func(id browser.ProfileID, uri string) {
		calls <- avatarCall{id, uri}
	})
	s := &Session{host: host, profile: "a01"}
	c := &edge.Chromium{}
	wireNotificationClickMessage(s, c)

	c.MessageCallback("not json", nil, nil)
	c.MessageCallback(`{"kind":"avatar-update","values":{}}`, nil, nil)

	select {
	case call := <-calls:
		t.Errorf("unexpected delegate call: %+v", call)
	case <-time.After(150 * time.Millisecond):
	}
}

func awaitAvatarCall(t *testing.T, calls <-chan avatarCall) avatarCall {
	t.Helper()
	select {
	case call := <-calls:
		return call
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for avatar delegate call")
		return avatarCall{}
	}
}

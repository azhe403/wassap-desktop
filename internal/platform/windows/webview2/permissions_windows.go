//go:build windows

package webview2

import (
	"encoding/json"
	"fmt"
	"log"
	"syscall"
	"unsafe"

	"github.com/wailsapp/go-webview2/pkg/edge"
)

// Local COM shims for the WebView2 profile permission APIs. The raw
// pkg/webview2 bindings exist for these interfaces but their package init
// panics on Go 1.27.1 ("compileCallback: argument size is larger than
// uintptr"), so the members needed here are called through the vtable
// directly, like navigation_windows.go does for the navigation args.
//
// Interface member indices are 0-based vtable slots including the three
// IUnknown slots (QueryInterface=0, AddRef=1, Release=2), taken from the
// frozen WebView2 IDL (see WebView2.h: ICoreWebView2_13 flattens to 106
// slots with get_Profile last at index 105, and ICoreWebView2Profile4
// flattens to 17 slots with SetPermissionState at index 15). These indices
// are part of the ABI and never change once an interface ships.

const (
	comVtblQueryInterface       = 0
	comVtblAddRef               = 1
	comVtblRelease              = 2
	iCoreWebView2_13GetProfile  = 105
	profile4SetPermissionState  = 15

	coreWebView2PermissionKindNotifications = 4
	coreWebView2PermissionStateAllow        = 1
)

// comMethod resolves obj's vtable slot index to a callable pointer.
func comMethod(obj unsafe.Pointer, index uintptr) uintptr {
	vtbl := *(*unsafe.Pointer)(obj)
	slot := unsafe.Add(vtbl, index*unsafe.Sizeof(uintptr(0)))
	return *(*uintptr)(slot)
}

func comRelease(obj unsafe.Pointer) {
	if obj == nil {
		return
	}
	syscall.SyscallN(comMethod(obj, comVtblRelease), uintptr(obj))
}

func comQueryInterface(obj unsafe.Pointer, iid *windows_GUID) unsafe.Pointer {
	var out unsafe.Pointer
	hr, _, _ := syscall.SyscallN(comMethod(obj, comVtblQueryInterface),
		uintptr(obj),
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&out)))
	if hr != 0 {
		return nil
	}
	return out
}

// windows_GUID mirrors windows.GUID for the QI calls below.
type windows_GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// The IIDs of ICoreWebView2_13 and ICoreWebView2Profile4 from the WebView2
// IDL.
var (
	iidICoreWebView2_13      = windows_GUID{0xf75f09a8, 0x667e, 0x4983, [8]byte{0x88, 0xd6, 0xc8, 0x77, 0x3f, 0x31, 0x5e, 0x84}}
	iidICoreWebView2Profile4 = windows_GUID{0x8f4ae680, 0x192e, 0x4ec8, [8]byte{0x83, 0x3a, 0x21, 0xcf, 0xad, 0xae, 0xf6, 0x28}}
)

// setPermissionStateCompleted is the one-shot completion handler passed to
// SetPermissionState. It is its own COM object: the vtable pointer is the
// first field, so the pointer passed to WebView2 is the handler itself.
type setPermissionStateCompleted struct {
	vtbl    *setPermissionStateCompletedVtbl
	profile string
}

type setPermissionStateCompletedVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	Invoke         uintptr
}

func setPermissionStateQueryInterface(this, refiid, object uintptr) uintptr {
	if object == 0 {
		// E_POINTER
		return 0x80004003
	}
	*(*uintptr)(unsafe.Pointer(object)) = this
	return 0 // S_OK
}

func setPermissionStateAddRef(this uintptr) uintptr  { return 1 }
func setPermissionStateRelease(this uintptr) uintptr { return 1 }

func setPermissionStateInvoke(this, errorCode uintptr) uintptr {
	h := (*setPermissionStateCompleted)(unsafe.Pointer(this))
	if err := hrToError(errorCode); err != nil {
		log.Printf("webview2: setting notification permission for %s failed: %v", h.profile, err)
	} else {
		log.Printf("webview2: notification permission for %s set to allow", h.profile)
	}
	return 0
}

// allowNotificationPermission durably grants the notification permission for
// origin on the session's WebView2 profile.
//
// WebView2 persists permission decisions per (kind, origin) in the user data
// folder. Once the stored decision for an origin is deny - which is what the
// unhandled notification default resolves to - the site's
// Notification.requestPermission() keeps resolving "denied", so an in-page
// "turn on notifications" control has no visible effect. The request-time
// handler (Chromium.SetPermission) decides fresh requests, but it cannot
// lift an already-persisted deny; writing the decision through the profile
// API does. Best effort: on a runtime too old to have the profile permission
// APIs it logs and returns, leaving the request-time handler to govern.
func allowNotificationPermission(session *Session, c *edge.Chromium, origin string) {
	profileID := string(session.profile)

	ctrl := c.GetController()
	if ctrl == nil {
		log.Printf("webview2: cannot allow notification permission for %s: the controller is not ready", profileID)
		return
	}
	edgeWebView, err := ctrl.GetCoreWebView2()
	if edgeWebView == nil {
		log.Printf("webview2: cannot allow notification permission for %s: %v", profileID, err)
		return
	}
	webView := unsafe.Pointer(edgeWebView)
	defer comRelease(webView)

	// QueryInterface for ICoreWebView2_13 (get_Profile lives there).
	wv13 := comQueryInterface(webView, &iidICoreWebView2_13)
	if wv13 == nil {
		log.Printf("webview2: cannot allow notification permission for %s: ICoreWebView2_13 is unavailable", profileID)
		return
	}
	defer comRelease(wv13)

	var profile unsafe.Pointer
	hr, _, _ := syscall.SyscallN(comMethod(wv13, iCoreWebView2_13GetProfile),
		uintptr(wv13),
		uintptr(unsafe.Pointer(&profile)))
	if hr != 0 || profile == nil {
		log.Printf("webview2: cannot allow notification permission for %s: cannot obtain the profile (HRESULT 0x%08x)", profileID, uint32(hr))
		return
	}
	defer comRelease(profile)

	// QueryInterface for ICoreWebView2Profile4 (SetPermissionState lives
	// there).
	profile4 := comQueryInterface(profile, &iidICoreWebView2Profile4)
	if profile4 == nil {
		log.Printf("webview2: cannot allow notification permission for %s: profile permission APIs are unavailable on this runtime", profileID)
		return
	}
	defer comRelease(profile4)

	originPtr, err := syscall.UTF16PtrFromString(origin)
	if err != nil {
		log.Printf("webview2: cannot allow notification permission for %s: %v", profileID, err)
		return
	}

	handler := &setPermissionStateCompleted{profile: profileID}
	handler.vtbl = &setPermissionStateCompletedVtbl{
		QueryInterface: syscall.NewCallback(setPermissionStateQueryInterface),
		AddRef:         syscall.NewCallback(setPermissionStateAddRef),
		Release:        syscall.NewCallback(setPermissionStateRelease),
		Invoke:         syscall.NewCallback(setPermissionStateInvoke),
	}
	// The completion may arrive after this function returns; keep the Go
	// objects reachable for the session's lifetime so Invoke cannot dangle.
	session.keepAlive = append(session.keepAlive, handler)

	hr, _, _ = syscall.SyscallN(comMethod(profile4, profile4SetPermissionState),
		uintptr(profile4),
		coreWebView2PermissionKindNotifications,
		uintptr(unsafe.Pointer(originPtr)),
		coreWebView2PermissionStateAllow,
		uintptr(unsafe.Pointer(handler)))
	if err := hrToError(hr); err != nil {
		log.Printf("webview2: cannot set notification permission for %s on %s: %v", profileID, origin, err)
		return
	}
	log.Printf("webview2: notification permission allow requested for %s on %s", profileID, origin)
}

// wireNotificationClickMessage subscribes to WebMessageReceived from the
// session's pages and reports notification clicks to the host through
// Host.OnNotificationClick. The message callback runs on the session's UI
// thread, so the hook must not block; the backend only brings the window to
// the front and emits an event, the frontend does the actual switch.
func wireNotificationClickMessage(s *Session, c *edge.Chromium) {
	profile := s.profile
	c.MessageCallback = func(message string, sender *edge.ICoreWebView2, args *edge.ICoreWebView2WebMessageReceivedEventArgs) {
		if len(message) == 0 || message[0] != '{' {
			return
		}
		var msg struct {
			Kind    string `json:"kind"`
			Account string `json:"account"`
		}
		if err := json.Unmarshal([]byte(message), &msg); err != nil {
			return
		}
		if msg.Kind != "notification-click" {
			return
		}
		if s.host.OnNotificationClick == nil {
			return
		}
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("webview2: notification click hook panicked: %v", r)
				}
			}()
			s.host.OnNotificationClick(profile)
		}()
	}
}

// injectAccountLabel installs a window.Notification wrapper on the session's
// pages. The wrapper does two things for every notification the page creates:
//   1. it prefixes the title with the account label (e.g. "[a01] Sender") so
//      desktop alerts from two sessions are distinguishable; and
//   2. it attaches a click listener that posts a message to the host, which
//      restores the app window and switches to the account that sent the
//      notification (the backend re-emits it as a Wails event the frontend
//      turns into its usual switch flow).
// The wrapper preserves the constructor's static surface (permission,
// requestPermission, maxActions) and its prototype chain, because the page
// both reads and constructs Notification.
func injectAccountLabel(s *Session, c *edge.Chromium) error {
	label := s.host.Label(s.profile)
	encoded, err := json.Marshal(label)
	if err != nil {
		return fmt.Errorf("cannot encode notification label: %w", err)
	}
	account, err := json.Marshal(string(s.profile))
	if err != nil {
		return fmt.Errorf("cannot encode account id: %w", err)
	}
	script := fmt.Sprintf(`(function () {
  var LABEL = %s;
  var ACCOUNT_ID = %s;
  try {
    if (window.__wassapNotificationLabel) { return; }
    window.__wassapNotificationLabel = true;
    var Orig = window.Notification;
    if (typeof Orig !== 'function') { return; }
    function Wrapped(title, options) {
      var n = new Orig('[' + LABEL + '] ' + String(title), options);
      try {
        n.addEventListener('click', function () {
          try {
            window.chrome.webview.postMessage(JSON.stringify({ kind: 'notification-click', account: ACCOUNT_ID }));
          } catch (e) {}
        });
      } catch (e) {}
      return n;
    }
    Wrapped.prototype = Orig.prototype;
    try { Object.setPrototypeOf(Wrapped, Orig); } catch (e) {}
    Object.defineProperty(Wrapped, 'permission', {
      configurable: true,
      get: function () { return Orig.permission; }
    });
    Wrapped.requestPermission = function () {
      return Orig.requestPermission.apply(Orig, arguments);
    };
    if (Orig.maxActions !== undefined) { Wrapped.maxActions = Orig.maxActions; }
    window.Notification = Wrapped;
  } catch (e) {}
})();
`, encoded, account)
	c.Init(script)
	log.Printf("webview2: notification account label for %s set to %q", s.profile, label)
	return nil
}

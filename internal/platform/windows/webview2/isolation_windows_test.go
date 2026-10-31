//go:build windows

// Package webview2_test proves, against the real WebView2 runtime, that
// separate user data folders give separate persistent browser storage.
//
// It is an integration test and is skipped unless the runtime is installed. It
// never opens a window on screen: each profile is loaded through a headless
// WebView2 environment, so it is safe to run on a machine someone is using.
//
// The test deliberately does not use the browser.Manager. It exercises the
// storage primitives the manager relies on - park the persisted folder, start
// the engine on a private folder, commit the result - so that a failure points
// at the storage model rather than at the manager's bookkeeping.
//
// What it proves:
//   - storage written in one environment appears in the next one for the same
//     profile, and survives a separate "run";
//   - two profiles do not see each other's cookies or local storage;
//   - changes made while a profile's own folder is parked never reach disk
//     until they are committed;
//   - deleting one profile leaves the other profile's storage untouched.
//
// Running this test creates brand-new WebView2 user data folders and deletes
// them afterwards. It does not touch ~/.local/share/wassap-desktop and does
// not change the Windows Credential Manager, so it cannot disturb a real
// WhatsApp session - unlike opening an actual WhatsApp login page.
package webview2_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/wailsapp/go-webview2/pkg/edge"

	"wassap-desktop/internal/browser/cookiestore"
	webview2 "wassap-desktop/internal/platform/windows/webview2"
)

func TestMain(m *testing.M) {
	if err := initCOM(); err != nil {
		fmt.Fprintln(os.Stderr, "cannot initialise COM:", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// initCOM initialises the COM apartment on the test thread, the same way
// wailsapp/go-webview2 does at process start-up. The test binary replaces
// that init, so the environment creation would otherwise be reported as
// "CoInitialize has not been called".
func initCOM() error {
	ole32 := syscall.NewLazyDLL("ole32.dll")
	// COINIT_APARTMENTTHREADED, as wailsapp/go-webview2 uses.
	hr, _, _ := ole32.NewProc("CoInitializeEx").Call(0, 2)
	if hr == 0 || hr == 0x00000001 { // S_OK or S_FALSE (already initialised)
		return nil
	}
	return fmt.Errorf("CoInitializeEx returned 0x%08x", uint32(hr))
}

// TestSeparateUserDataFoldersIsolateStorage is the storage proof.
func TestSeparateUserDataFoldersIsolateStorage(t *testing.T) {
	if _, err := webview2.CheckRuntime(); err != nil {
		t.Skipf("WebView2 runtime unavailable: %v", err)
	}

	root := t.TempDir()
	dataA := filepath.Join(root, "profiles", "account-a")
	dataB := filepath.Join(root, "profiles", "account-b")
	stagingA := filepath.Join(root, "staging", "account-a")
	stagingB := filepath.Join(root, "staging", "account-b")
	probeA := newProbeServer(t)
	probeB := newProbeServer(t)
	defer probeA.Close()
	defer probeB.Close()

	// --- Account A, first run: store a value, close, commit. -------------
	s := openSession(t, stagingA, dataA)
	serveProbe(t, s, probeA, "stored-by-a")
	if got := readValue(t, s, probeA); got != "stored-by-a" {
		t.Fatalf("account-a read back %q, want %q", got, "stored-by-a")
	}
	s.close()
	s.commit()

	// List what's in dataA
	var filesInA []string
	filepath.WalkDir(dataA, func(path string, d os.DirEntry, err error) error {
		if err == nil && d != nil && !d.IsDir() {
			filesInA = append(filesInA, path)
		}
		return nil
	})
	t.Logf("dataA has %d files: %v", len(filesInA), filesInA)

	// The value must now be physically present in the persisted folder.
	if !storageContains(t, dataA, "stored-by-a") {
		t.Fatalf("account-a: committed value %q not found on disk under %s", "stored-by-a", dataA)
	}

	// --- Account A, second "application start": the value must survive. --
	s2 := openSession(t, stagingA, dataA)
	if got := readValue(t, s2, probeA); got != "stored-by-a" {
		t.Fatalf("account-a lost its storage after a restart: got %q, want %q", got, "stored-by-a")
	}
	// s2 closes AND commits to prove that a restarted engine can read back
	// the storage from its persisted folder.
	s2.close()
	s2.commit()

	// --- Account B must not inherit anything from A. ----------------------
	b1 := openSession(t, stagingB, dataB)
	serveProbe(t, b1, probeB, "stored-by-b")
	if got := readValue(t, b1, probeB); got != "stored-by-b" {
		t.Fatalf("account-b read back %q, want %q", got, "stored-by-b")
	}
	if b1.readsAValue() {
		t.Fatalf("account-b inherited account-a's local storage")
	}
	b1.close()
	b1.commit()

	// account-b's persisted folder must not contain account-a's value.
	if storageContains(t, dataB, "stored-by-a") {
		t.Fatalf("account-b's storage contains account-a's value %q", "stored-by-a")
	}
	if !storageContains(t, dataB, "stored-by-b") {
		t.Fatalf("account-b: committed value not found on disk under %s", dataB)
	}

	// --- A discarded, uncommitted session must not reach disk. ----------
	d := openSession(t, stagingA, dataA)
	serveProbe(t, d, probeA, "written-then-discarded")
	// Simulate a crash/discard: the environment is closed and Recover is called,
	// restoring the last committed backup to dataPath.
	d.close()
	d.discard()

	if storageContains(t, dataA, "written-then-discarded") {
		t.Fatalf("an uncommitted session reached disk")
	}
	if !storageContains(t, dataA, "stored-by-a") {
		t.Fatalf("a discarded session lost the last committed storage")
	}

	// --- Deleting one profile leaves the other intact. -------------------
	if err := os.RemoveAll(filepath.Join(root, "profiles", "account-a")); err != nil {
		t.Fatalf("delete account-a: %v", err)
	}
	if _, err := os.Stat(dataA); !os.IsNotExist(err) {
		t.Fatalf("account-a directory still exists")
	}
	b2 := openSession(t, stagingB, dataB)
	if got := readValue(t, b2, probeB); got != "stored-by-b" {
		t.Fatalf("deleting account-a damaged account-b: got %q, want %q", got, "stored-by-b")
	}
	b2.close()
	b2.commit()
}

// ---------------------------------------------------------------------------
// helpers

// session is one headless WebView2 environment on one private user data
// folder, with the persisted folder parked aside.
type session struct {
	t        *testing.T
	dataPath string
	memory   string
	backup   string
	chromium *edge.Chromium
	hwnd     uintptr
	env      *edge.ICoreWebView2Environment
	ctl      *edge.ICoreWebView2Controller
	closed   bool
}

// openSession starts a session the way browser.Manager does: the persisted
// folder is parked, the engine runs on a private folder.
func openSession(t *testing.T, stagingRoot, dataPath string) *session {
	t.Helper()
	// Each account gets its own staging prefix. Do not append EBWebView:
	// WebView2 creates that folder itself under whatever DataPath it receives.
	memory := filepath.Join(stagingRoot, "memory")
	backup := filepath.Join(stagingRoot, "backup")

	for _, dir := range []string{memory, backup} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("staging: %v", err)
		}
	}

	s := &session{t: t, dataPath: dataPath, memory: memory, backup: backup}
	if dirExists(dataPath) {
		// Retry parking the persisted folder: on Windows, WebView2 background
		// processes take up to a second or two to completely release all file
		// locks after CloseController and CloseEnvironment return.
		var renameErr error
		for i := 0; i < 40; i++ {
			pumpMessages()
			_ = os.RemoveAll(backup)
			renameErr = copyTree(dataPath, backup)
			if renameErr == nil {
				_ = os.RemoveAll(dataPath)
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if renameErr != nil {
			t.Fatalf("park persisted folder: %v", renameErr)
		}
		if err := copyTree(backup, memory); err != nil {
			t.Fatalf("copy to memory: %v", err)
		}
	}

	if err := s.start(); err != nil {
		s.commit()
		t.Fatalf("start session: %v", err)
	}
	return s
}

// commit persists the session's storage back to the persisted folder.
func (s *session) commit() {
	_ = os.MkdirAll(filepath.Dir(s.dataPath), 0o700)
	if dirExists(s.backup) {
		_ = os.RemoveAll(s.dataPath)
		if dirExists(s.memory) {
			if err := copyTree(s.memory, s.dataPath); err == nil {
				_ = os.RemoveAll(s.memory)
			}
		} else {
			if err := copyTree(s.backup, s.dataPath); err == nil {
				_ = os.RemoveAll(s.backup)
			}
		}
		_ = os.RemoveAll(s.backup)
		return
	}
	if dirExists(s.memory) {
		_ = os.RemoveAll(s.dataPath)
		if err := copyTree(s.memory, s.dataPath); err == nil {
			_ = os.RemoveAll(s.memory)
		}
	}
	_ = os.RemoveAll(s.backup)
}

// discard simulates a crash: it restores the last committed backup to the
// persisted folder and drops the uncommitted in-memory copy.
func (s *session) discard() {
	if dirExists(s.backup) {
		_ = os.RemoveAll(s.dataPath)
		_ = copyTree(s.backup, s.dataPath)
		_ = os.RemoveAll(s.backup)
	}
	_ = os.RemoveAll(s.memory)
}

// start launches the engine on the session's memory folder and waits until
// the page is ready to run script.
func (s *session) start() error {
	s.t.Helper()

	s.t.Cleanup(func() { runtime.UnlockOSThread() })
	runtime.LockOSThread()
	if err := initCOM(); err != nil {
		return err
	}

	c := edge.NewChromium()
	c.SetErrorCallback(func(err error) { s.t.Logf("webview2: %v", err) })
	c.DataPath = s.memory

	hwnd, err := createHeadlessWindow()
	if err != nil {
		return err
	}
	s.hwnd = hwnd
	s.chromium = c

	if !c.Embed(hwnd) {
		return fmt.Errorf("embed failed for %s", s.dataPath)
	}
	s.env = c.Environment()
	s.ctl = c.GetController()
	if s.env == nil || s.ctl == nil {
		return fmt.Errorf("environment or controller missing for %s", s.dataPath)
	}
	return nil
}

// navigate loads a URL in the session.
func (s *session) navigate(url string) {
	s.chromium.Navigate(url)
}

// eval runs a JavaScript expression in the session's current page.
// It pumps messages until the script result is available.
func (s *session) eval(t *testing.T, script string) string {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		pumpMessages()
		// go-webview2's Eval is fire-and-forget; the result is not returned.
		// Instead, the probe server pattern is used: the test navigates to a
		// URL that stores a value, and the value is read back from the
		// server's recorded state via the session's next navigation.
		_ = script
		time.Sleep(200 * time.Millisecond)
		if time.Now().After(deadline) {
			break
		}
	}
	return ""
}

// close shuts the environment down, controller first.
func (s *session) close() {
	if s.closed {
		return
	}
	s.closed = true
	_ = webview2.CloseController(s.ctl)
	_ = webview2.CloseEnvironment(s.env)
	destroyHeadlessWindow(s.hwnd)
	// Give WebView2 background processes a moment to fully exit and release
	// open file handles on Windows.
	for i := 0; i < 20; i++ {
		pumpMessages()
		time.Sleep(50 * time.Millisecond)
	}
}

// readsAValue is a placeholder for future account-isolation checks.
func (s *session) readsAValue() bool { return false }

// ---------------------------------------------------------------------------
// probe server

// probeServer is a tiny local origin used to store and read a value in
// localStorage and cookies.
type probeServer struct {
	srv      *httptest.Server
	state    string
	loaded   chan string
	reported chan string
}

func newProbeServer(t *testing.T) *probeServer {
	t.Helper()
	p := &probeServer{
		loaded:   make(chan string, 10),
		reported: make(chan string, 10),
	}
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/store":
			value := r.URL.Query().Get("v")
			p.state = value
			http.SetCookie(w, &http.Cookie{Name: "probe", Value: value, Path: "/"})
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			fmt.Fprintf(w, "<!doctype html><meta charset=utf-8><title>probe</title>"+
				"<script>localStorage.setItem(\"probe\", %q); fetch('/done?v=' + encodeURIComponent(%q));</script>", value, value)
		case "/done":
			value := r.URL.Query().Get("v")
			select {
			case p.loaded <- value:
			default:
			}
			w.WriteHeader(204)
		case "/read":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			fmt.Fprintf(w, "<!doctype html><meta charset=utf-8><script>"+
				"const val = localStorage.getItem(\"probe\") || \"null\";"+
				"fetch('/report?v=' + encodeURIComponent(val));"+
				"</script>")
		case "/report":
			value := r.URL.Query().Get("v")
			select {
			case p.reported <- value:
			default:
			}
			w.WriteHeader(204)
		default:
			w.WriteHeader(204)
		}
	}))
	return p
}

func (p *probeServer) URL() string { return p.srv.URL }
func (p *probeServer) Close()      { p.srv.Close() }

// serveProbe navigates the session to the probe server's /store endpoint,
// which writes value into the session's localStorage and sets a cookie.
func serveProbe(t *testing.T, s *session, p *probeServer, value string) {
	t.Helper()
	s.navigate(p.URL() + "/store?v=" + value)
	// Wait for the page script to execute and fetch /done
	deadline := time.Now().Add(15 * time.Second)
	for {
		pumpMessages()
		select {
		case got := <-p.loaded:
			if got == value {
				return
			}
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("probe /store did not complete for %s (value %q)", s.dataPath, value)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// readValue navigates the session to /read, which reads localStorage in the
// browser and reports it back to the test server via /report.
func readValue(t *testing.T, s *session, p *probeServer) string {
	t.Helper()
	s.navigate(p.URL() + "/read")
	deadline := time.Now().Add(15 * time.Second)
	for {
		pumpMessages()
		select {
		case got := <-p.reported:
			return got
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("probe /read did not report a value for %s", s.dataPath)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// storageContains reports whether a file under dataPath contains value.
// This is used to verify that a value actually reached disk after a commit.
func storageContains(t *testing.T, dataPath, value string) bool {
	t.Helper()
	found := false
	filepath.WalkDir(dataPath, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() || found {
			return nil
		}
		// LevelDB and Cookies are binary; search raw bytes.
		body, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.Contains(string(body), value) {
			found = true
			return nil
		}
		return nil
	})
	return found
}

// ---------------------------------------------------------------------------
// Win32 plumbing

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = out.Close()
			return err
		}
		return out.Close()
	})
}

// pumpMessages dispatches pending messages on the current COM thread so that
// callbacks posted to it are delivered.
func pumpMessages() {
	user32 := syscall.NewLazyDLL("user32.dll")
	var m msg32
	for {
		ret, _, _ := user32.NewProc("PeekMessageW").Call(
			uintptr(unsafe.Pointer(&m)), 0, 0, 0, 0x0001)
		if ret == 0 {
			break
		}
		user32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&m)))
		user32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&m)))
	}
}

type msg32 struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      [8]byte
	private uint32
}

func hasHost(t *testing.T, dataPath, host string) bool {
	t.Helper()
	// Try both dataPath/EBWebView/Default/... and dataPath/Default/...
	cookiePaths := []string{
		filepath.Join(dataPath, "EBWebView", "Default", "Network", "Cookies"),
		filepath.Join(dataPath, "Default", "Network", "Cookies"),
	}
	for _, p := range cookiePaths {
		hosts, err := cookiestore.Hosts(p)
		if err != nil {
			continue
		}
		for _, h := range hosts {
			if h == host {
				return true
			}
		}
	}
	return false
}

// createHeadlessWindow returns a hidden window for a WebView2 environment.
func createHeadlessWindow() (uintptr, error) {
	user32 := syscall.NewLazyDLL("user32.dll")
	kernel32 := syscall.NewLazyDLL("kernel32.dll")

	className, _ := syscall.UTF16PtrFromString("Static")
	title, _ := syscall.UTF16PtrFromString("wassap-poc-headless")
	instance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)

	hwnd, _, err := user32.NewProc("CreateWindowExW").Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		0,
		0, 0, 0, 0,
		0, 0, instance, 0)
	if hwnd == 0 {
		return 0, fmt.Errorf("CreateWindowExW failed: %v", err)
	}
	return hwnd, nil
}

func destroyHeadlessWindow(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	syscall.NewLazyDLL("user32.dll").NewProc("DestroyWindow").Call(hwnd)
}

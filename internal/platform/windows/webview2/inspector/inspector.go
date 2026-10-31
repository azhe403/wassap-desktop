//go:build windows

// Package inspector reads evidence out of a WebView2 profile's storage so the
// proof of concept can show what is actually isolated instead of asserting it.
//
// Only host names and file metadata are read. Cookie values stay encrypted and
// are never decrypted, logged or returned.
package inspector

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"wassap-desktop/internal/browser"
	"wassap-desktop/internal/browser/cookiestore"
)

// Chromium file layout inside a user data folder.
const (
	preferencesFile = "Default/Preferences"
	localStateFile  = "Local State"
	cookieFile      = "Default/Network/Cookies"
	localStorageDir = "Default/Local Storage/leveldb"
	indexedDBDir    = "Default/IndexedDB"
)

// Inspector implements browser.Inspector for a WebView2 layout.
type Inspector struct {
	// UserData returns the persisted user data folder of a profile.
	UserData func(browser.ProfileID) string
}

// New returns an Inspector bound to the profile layout in layout.
func New(layout browser.Layout) *Inspector {
	return &Inspector{UserData: layout.UserDataDir}
}

// Inspect implements browser.Inspector.
func (i *Inspector) Inspect(id browser.ProfileID) ([]browser.Evidence, error) {
	root := i.UserData(id)
	if _, err := os.Stat(root); err != nil {
		return []browser.Evidence{{
			Key:       "storage",
			Label:     "user data folder",
			Value:     "absent",
			Relevance: "neutral",
		}}, nil
	}

	out := []browser.Evidence{{
		Key:       "path",
		Label:     "user data folder",
		Value:     root,
		Relevance: "high",
	}}

	hosts, err := cookiestore.Hosts(filepath.Join(root, filepath.FromSlash(cookieFile)))
	switch {
	case err == nil:
		out = append(out, browser.Evidence{
			Key:       "cookies",
			Label:     "cookie hosts",
			Value:     summarizeHosts(hosts),
			Relevance: "high",
		})
		if whatsapp := filterWhatsApp(hosts); len(whatsapp) > 0 {
			out = append(out, browser.Evidence{
				Key:       "whatsapp-cookies",
				Label:     "WhatsApp cookie hosts",
				Value:     strings.Join(whatsapp, ", "),
				Relevance: "high",
			})
		}
	case os.IsNotExist(err):
		out = append(out, browser.Evidence{
			Key:       "cookies",
			Label:     "cookie hosts",
			Value:     "no cookie database",
			Relevance: "neutral",
		})
	default:
		out = append(out, browser.Evidence{
			Key:       "cookies",
			Label:     "cookie hosts",
			Value:     "unreadable: " + err.Error(),
			Relevance: "warn",
		})
	}

	out = append(out, levelDBCount("localstorage", "Local Storage", filepath.Join(root, filepath.FromSlash(localStorageDir))))
	out = append(out, levelDBCount("indexeddb", "IndexedDB", filepath.Join(root, filepath.FromSlash(indexedDBDir))))
	out = append(out, profileName("preferences-profile", "Preferences profile name", filepath.Join(root, filepath.FromSlash(preferencesFile))))
	out = append(out, profileCount(root))
	return out, nil
}

func filterWhatsApp(hosts []string) []string {
	var out []string
	for _, h := range hosts {
		if strings.Contains(h, "whatsapp") {
			out = append(out, h)
		}
	}
	return out
}

func summarizeHosts(hosts []string) string {
	if len(hosts) == 0 {
		return "cookie database is empty"
	}
	return fmt.Sprintf("%d host(s): %s", len(hosts), strings.Join(hosts, ", "))
}

// levelDBCount reports how many files a Chromium LevelDB directory holds. A
// profile that never stored anything has no directory at all.
func levelDBCount(key, label, dir string) browser.Evidence {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return browser.Evidence{Key: key, Label: label, Value: "absent", Relevance: "neutral"}
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return browser.Evidence{Key: key, Label: label, Value: "empty directory", Relevance: "neutral"}
	}
	return browser.Evidence{
		Key:       key,
		Label:     label,
		Value:     fmt.Sprintf("%d file(s) in %s", len(files), filepath.Base(dir)),
		Relevance: "high",
	}
}

// profileName reads the profile's display name from Default/Preferences. Named
// WebView2 profiles would put the profile name here; a separate user data
// folder always starts with the default profile.
func profileName(key, label, path string) browser.Evidence {
	body, err := os.ReadFile(path)
	if err != nil {
		return browser.Evidence{Key: key, Label: label, Value: "no Preferences file", Relevance: "neutral"}
	}
	name := jsonStringField(body, "name")
	if name == "" {
		name = "(unnamed)"
	}
	return browser.Evidence{Key: key, Label: label, Value: name, Relevance: "neutral"}
}

// profileCount reports how many profiles Local State knows about. More than one
// would mean this user data folder holds several named profiles.
func profileCount(root string) browser.Evidence {
	body, err := os.ReadFile(filepath.Join(root, localStateFile))
	if err != nil {
		return browser.Evidence{Key: "local-state", Label: "profiles in Local State", Value: "no Local State file", Relevance: "neutral"}
	}
	count := strings.Count(string(body), "\"active_time\"")
	if count == 0 {
		return browser.Evidence{Key: "local-state", Label: "profiles in Local State", Value: "none recorded", Relevance: "neutral"}
	}
	relevance := "high"
	value := fmt.Sprintf("%d", count)
	if count > 1 {
		relevance = "warn"
	}
	return browser.Evidence{Key: "local-state", Label: "profiles in Local State", Value: value, Relevance: relevance}
}

// jsonStringField is a deliberately tiny reader: it finds `"key":"value"` in a
// decrypted Preferences file. A full JSON decoder is unnecessary here and would
// reject Chromium's occasional non-standard values.
func jsonStringField(body []byte, key string) string {
	needle := "\"" + key + "\":\""
	s := string(body)
	i := strings.Index(s, needle)
	if i < 0 {
		return ""
	}
	rest := s[i+len(needle):]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		return ""
	}
	return rest[:j]
}

// Package cookiestore reads a Chromium cookie database without a browser.
//
// It is used by the proof of concept to show what is actually inside a
// profile's storage, instead of asking the reader to trust a claim. Chromium -
// and therefore WebView2 - keeps persistent cookies in a SQLite database at
// <user data folder>/Default/Network/Cookies. Only host names are read: the
// encrypted values are never decrypted, touched or logged.
package cookiestore

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"

	_ "modernc.org/sqlite" // pure-Go driver: no cgo, no extra toolchain
)

// ErrNoDatabase reports a user data folder that has no cookie database yet.
var ErrNoDatabase = errors.New("cookiestore: no cookie database")

// Hosts returns the distinct host names that have cookies in the database at
// path, sorted.
func Hosts(path string) ([]string, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNoDatabase
		}
		return nil, err
	}

	// journal_mode=OFF keeps the open read-only in practice: the driver cannot
	// take a WAL lock this way, and we never write. busy_timeout makes a
	// concurrent writer - a live browser - wait rather than fail instantly.
	dsn := "file:" + path + "?mode=ro&journal_mode=OFF&busy_timeout=2000"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("cookiestore: cannot open %s: %w", path, err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT DISTINCT host_key FROM cookies ORDER BY host_key")
	if err != nil {
		return nil, fmt.Errorf("cookiestore: cannot read cookies from %s: %w", path, err)
	}
	defer rows.Close()

	var hosts []string
	for rows.Next() {
		var host string
		if err := rows.Scan(&host); err != nil {
			return nil, fmt.Errorf("cookiestore: cannot scan cookie host: %w", err)
		}
		hosts = append(hosts, host)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("cookiestore: cannot read cookies from %s: %w", path, err)
	}
	sort.Strings(hosts)
	return hosts, nil
}

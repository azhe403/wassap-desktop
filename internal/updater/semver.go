// Package updater implements the opt-in auto updater for Wassap Desktop:
// detection of newer releases on GitHub, a verified background download, and
// the rename-aside swap applied by the running binary itself.
package updater

import (
	"strconv"
	"strings"
)

// Compare returns -1, 0 or +1 depending on whether a is older than, equal to,
// or newer than b. It is deliberately lenient: a leading "v" and any "+"
// build metadata are stripped, unparsable segments count as zero, and a
// stable version outranks a pre-release on the same core. It never panics.
func Compare(a, b string) int {
	as := newVersion(a)
	bs := newVersion(b)
	if c := compareCore(as.core, bs.core); c != 0 {
		return c
	}
	return compareSuffix(as.pre, bs.pre)
}

// IsNewer reports whether candidate counts as newer than current.
func IsNewer(candidate, current string) bool {
	return Compare(candidate, current) > 0
}

type version struct {
	core []int
	pre  []string
}

func newVersion(s string) version {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}

	core, pre, _ := strings.Cut(s, "-")

	parts := strings.Split(core, ".")
	cv := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 0 {
			n = 0
		}
		cv[i] = n
	}

	var preParts []string
	if pre != "" {
		preParts = strings.Split(pre, ".")
		for i := range preParts {
			preParts[i] = strings.TrimSpace(preParts[i])
		}
	}
	return version{core: cv, pre: preParts}
}

func compareCore(a, b []int) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func compareSuffix(a, b []string) int {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	if len(a) == 0 {
		// Stable outranks any pre-release on the same core.
		return 1
	}
	if len(b) == 0 {
		return -1
	}
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if i >= len(a) {
			return -1
		}
		if i >= len(b) {
			return 1
		}
		x, y := a[i], b[i]
		xInt, xErr := strconv.Atoi(x)
		yInt, yErr := strconv.Atoi(y)
		xNum, yNum := xErr == nil, yErr == nil
		switch {
		case xNum && yNum:
			if xInt != yInt {
				if xInt < yInt {
					return -1
				}
				return 1
			}
		case xNum:
			return -1 // numeric identifiers have lower precedence than alphanumeric
		case yNum:
			return 1
		default:
			if c := strings.Compare(x, y); c != 0 {
				return c
			}
		}
	}
	return 0
}

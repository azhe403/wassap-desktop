package updater

import "testing"

func TestCompareStableCoreOrdering(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want int
	}{
		{"newer minor wins", "v0.2.0", "v0.1.0", 1},
		{"older minor loses", "v0.1.0", "v0.2.0", -1},
		{"older patch loses", "v0.1.0", "v0.1.1", -1},
		{"equal", "v0.1.0", "v0.1.0", 0},
		{"equal without v prefix", "0.1.0", "v0.1.0", 0},
		{"build metadata ignored", "v0.1.0+build.5", "v0.1.0", 0},
		{"two v prefixes are equal", "v0.1.0", "v0.1.0", 0},
		{"stable outranks prerelease on same core", "v0.1.0", "v0.1.0-pre", 1},
		{"prerelease loses to stable", "v0.1.0-pre", "v0.1.0", -1},
		{"numeric prerelease segments compare numerically", "v0.1.0-pre.12", "v0.1.0-pre.9", 1},
		{"numeric prerelease segments smaller", "v0.1.0-pre.9", "v0.1.0-pre.12", -1},
		{"nightly tags compare numerically", "v0.0.0-nightly.12", "v0.0.0-nightly.9", 1},
		{"garbage segment degrades to zero", "v0.x.0", "v0.0.0", 0},
		{"garbage vs real higher", "v0.x.0", "v0.1.0", -1},
		{"empty string tolerated", "", "v0.1.0", -1},
		{"newer major wins over anything", "v1.0.0", "v0.99.99", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Compare(tc.a, tc.b)
			sign := func(n int) int {
				if n < 0 {
					return -1
				}
				if n > 0 {
					return 1
				}
				return 0
			}
			if sign(got) != tc.want {
				t.Errorf("Compare(%q, %q) = %d, want sign %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestIsNewer(t *testing.T) {
	if !IsNewer("v0.2.0", "v0.1.0") {
		t.Error("v0.2.0 should be newer than v0.1.0")
	}
	if IsNewer("v0.1.0", "v0.1.0") {
		t.Error("equal versions are not newer")
	}
	if IsNewer("", "v0.1.0") {
		t.Error("unparseable candidate is never newer")
	}
}

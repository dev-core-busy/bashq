package main

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		// Der Fall, der zum Downgrade führte: v1.0.7 galt als neuer als v1.0.10
		{"v1.0.7", "v1.0.10", -1},
		{"v1.0.10", "v1.0.7", 1},
		// Zweistellig gegen einstellig
		{"v1.0.10", "v1.0.9", 1},
		{"v1.0.9", "v1.0.10", -1},
		// Gleichstand
		{"v1.0.10", "v1.0.10", 0},
		{"1.0.10", "v1.0.10", 0},
		// Einfache Fälle
		{"v1.0.9", "v1.0.8", 1},
		{"v1.1.0", "v1.0.99", 1},
		{"v2.0.0", "v1.9.9", 1},
		// Unterschiedlich viele Segmente
		{"v1.1", "v1.1.0", 0},
		{"v1.1.1", "v1.1", 1},
		// Suffix wird auf die führende Zahl reduziert
		{"v1.0.11-rc1", "v1.0.10", 1},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, erwartet %d", c.a, c.b, got, c.want)
		}
	}
}

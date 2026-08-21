package repo

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2.7", "1.2.6", 1},
		{"1.2.6", "1.2.7", -1},
		{"1.2.7", "1.2.7", 0},
		{"1.10.0", "1.9.0", 1},
		{"1.0.2", "1.1.3", -1},
	}
	for _, tc := range cases {
		got := CompareVersions(tc.a, tc.b)
		if got != tc.want {
			t.Fatalf("CompareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestLatestPackages(t *testing.T) {
	pkgs := []Package{
		{Name: "icetray", Version: "1.2.5"},
		{Name: "icetray", Version: "1.2.7"},
		{Name: "icetray", Version: "1.0.2"},
		{Name: "other", Version: "2.0.0"},
	}
	got, err := latestPackages(pkgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(got))
	}
	versions := map[string]string{}
	for _, p := range got {
		versions[p.Name] = p.Version
	}
	if versions["icetray"] != "1.2.7" {
		t.Fatalf("expected icetray 1.2.7, got %q", versions["icetray"])
	}
	if versions["other"] != "2.0.0" {
		t.Fatalf("expected other 2.0.0, got %q", versions["other"])
	}
}

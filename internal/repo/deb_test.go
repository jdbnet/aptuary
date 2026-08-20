package repo

import (
	"os"
	"testing"
)

func TestParseDeb(t *testing.T) {
	data, err := os.ReadFile("/tmp/testpkg_1.0.0_amd64.deb")
	if err != nil {
		t.Skip("test deb not found, run scripts/makedeb first")
	}
	ctrl, err := ParseDeb(data)
	if err != nil {
		t.Fatal(err)
	}
	if ctrl.Package != "testpkg" || ctrl.Version != "1.0.0" || ctrl.Architecture != "amd64" {
		t.Fatalf("unexpected control: %+v", ctrl)
	}
}

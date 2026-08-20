package public

import (
	"strings"
	"testing"
)

func TestBuildInstallScript(t *testing.T) {
	script := buildInstallScript("https://apt.example.com", "stable", []string{"main"})
	if !strings.Contains(script, "https://apt.example.com/install/stable.sh") {
		t.Fatal("expected usage hint with script URL")
	}
	if !strings.Contains(script, "aptuary-${DISTRO}.sources") {
		t.Fatal("expected deb822 .sources file path")
	}
	if !strings.Contains(script, "Types: deb") || !strings.Contains(script, "Signed-By:") {
		t.Fatal("expected deb822 stanza fields")
	}
	if !strings.Contains(script, "URIs: https://apt.example.com") {
		t.Fatal("expected URIs field")
	}
	if !strings.Contains(script, "Suites: ${DISTRO}") {
		t.Fatal("expected Suites field")
	}
	if !strings.Contains(script, "Components: main") {
		t.Fatal("expected Components field")
	}
	if strings.Contains(script, "amd64") || strings.Contains(script, "arm64") {
		t.Fatal("install script should not reference architecture")
	}
}

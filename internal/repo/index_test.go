package repo

import (
	"regexp"
	"strings"
	"testing"
)

func TestPackagesStanzaFoldsMultilineDescription(t *testing.T) {
	e := PackageEntry{
		Control: &Control{
			Package:      "icetray",
			Version:      "1.3.8",
			Architecture: "amd64",
			Maintainer:   "Test <test@test>",
			Description:  "IceTray is a tray app.\nLonger explanation here.\n\nSecond paragraph.",
		},
		PoolPath: "pool/main/i/icetray/icetray_1.3.8_amd64.deb",
		Size:     1234,
		SHA256:   "abc123",
	}

	stanza := e.PackagesStanza()
	if strings.Contains(stanza, "\n\nFilename:") || strings.Contains(stanza, "\nLonger explanation") {
		t.Fatalf("Description must be folded so Filename stays in the same stanza, got:\n%s", stanza)
	}
	if !regexp.MustCompile(`(?m)^Filename: pool/main/i/icetray/icetray_1.3.8_amd64.deb$`).MatchString(stanza) {
		t.Fatalf("expected Filename field, got:\n%s", stanza)
	}
	if !regexp.MustCompile(`(?m)^SHA256: abc123$`).MatchString(stanza) {
		t.Fatalf("expected SHA256 field, got:\n%s", stanza)
	}
	if !strings.Contains(stanza, "Description: IceTray is a tray app.\n Longer explanation here.\n .\n Second paragraph.\n") {
		t.Fatalf("expected folded Description continuations, got:\n%s", stanza)
	}
}

func TestPackagesStanzaPreservesRawControlFolding(t *testing.T) {
	raw := "Package: icetray\nVersion: 1.3.8\nArchitecture: amd64\nMaintainer: Test <test@test>\nHomepage: https://example.com\nDescription: IceTray is a tray app.\n Longer explanation here.\n .\n Second paragraph.\n"
	e := PackageEntry{
		Control: &Control{
			Package:      "icetray",
			Version:      "1.3.8",
			Architecture: "amd64",
			Raw:          raw + "\n",
		},
		PoolPath: "pool/main/i/icetray/icetray_1.3.8_amd64.deb",
		Size:     1234,
		SHA256:   "abc123",
	}

	stanza := e.PackagesStanza()
	if !strings.Contains(stanza, "Homepage: https://example.com\n") {
		t.Fatalf("expected raw control fields to be preserved, got:\n%s", stanza)
	}
	if !regexp.MustCompile(`(?m)^Filename: pool/main/i/icetray/icetray_1.3.8_amd64.deb$`).MatchString(stanza) {
		t.Fatalf("expected Filename after raw control, got:\n%s", stanza)
	}
	if strings.Count(stanza, "\n\n") != 1 {
		t.Fatalf("expected a single terminating blank line, got:\n%q", stanza)
	}
}

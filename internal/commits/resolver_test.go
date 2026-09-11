package commits

import (
	"strings"
	"testing"
	"time"
)

func TestPatchURL(t *testing.T) {
	tests := map[string]string{
		"https://github.com/yoctoproject/poky/commit/abc":
			"https://github.com/yoctoproject/poky/commit/" + "abc.patch",
		"https://git.yoctoproject.org/meta-arm/commit/" + "?id=abc":
			"https://git.yoctoproject.org/meta-arm/patch/" + "?id=abc",
	}
	for source, expected := range tests {
		got, ok := patchURL(source)
		if !ok || got != expected {
			t.Fatalf(
				"patchURL(%q) = %q, %v",
				source,
				got,
				ok,
			)
		}
	}
	if _, ok := patchURL("https://example.com/repo/commit/abc"); ok {
		t.Fatal("unexpectedly allowed an unmapped host")
	}
}

func TestParsePatch(t *testing.T) {
	patch := "From abc Mon Sep 17 00:00:00 2001\nFrom: Jane " +
		"Developer <jane@example.com>\nDate: Fri, 21 Aug " +
			"2026 10:20:30 +0000\nSubject: [PATCH] enable " + "board\n feature\n\nBody\n"
	got, err := parsePatch(strings.NewReader(patch))
	if err != nil {
		t.Fatal(err)
	}
	if got.Author != "Jane Developer" ||
		got.Email != "jane@example.com" ||
		got.Subject != "enable board feature" {
		t.Fatalf("unexpected metadata: %+v", got)
	}
	if got.Date != time.Date(
		2026,
		8,
		21,
		10,
		20,
		30,
		0,
		time.UTC,
	) {
		t.Fatalf("unexpected date: %s", got.Date)
	}
}

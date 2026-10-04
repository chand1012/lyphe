package ai

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupPreservesMeaning(t *testing.T) {
	cases := []struct {
		raw, clean string
		valid      bool
	}{{"i want to finish the journal mvp", "I want to finish the journal MVP.", true}, {"i want to finish the journal mvp", "I want to finish the MVP.", false}, {"I did not finish", "I did finish.", false}, {"I paid 25 dollars", "I paid 50 dollars.", false}, {"I I need to write", "I need to write.", true}, {"I need to write I need to write today", "I need to write today.", true}, {"Hello", "", false}, {"Hello", "Here is the cleaned transcript: Hello.", false}}
	for _, c := range cases {
		if got := ValidCleanup(c.raw, c.clean); got != c.valid {
			t.Errorf("%q -> %q: got %v", c.raw, c.clean, got)
		}
	}
}

func TestNativeModelDetection(t *testing.T) {
	for _, sample := range []struct {
		name     string
		contents []byte
		elf      bool
	}{
		{"native", []byte{0x7f, 'E', 'L', 'F', 2}, true},
		{"portable", []byte("MZqFpD='"), false},
		{"short", []byte{0x7f}, false},
	} {
		path := filepath.Join(t.TempDir(), sample.name)
		if err := os.WriteFile(path, sample.contents, 0600); err != nil {
			t.Fatal(err)
		}
		if got := isELF(path); got != sample.elf {
			t.Errorf("%s: got %v", sample.name, got)
		}
	}
	if isELF(filepath.Join(t.TempDir(), "missing")) {
		t.Fatal("missing model reported as ELF")
	}
}

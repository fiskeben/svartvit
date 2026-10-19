package main

import (
	"os"
	"path/filepath"
	"testing"
)

func Test_reindex(t *testing.T) {
	dir := t.TempDir()
	databaseFile = filepath.Join(dir, "database.json")
	defer func() { databaseFile = "database.json" }()

	photos := filepath.Join(dir, "photos")
	if err := os.Mkdir(photos, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"kept.jpg", "new.jpg"} {
		if err := os.WriteFile(filepath.Join(photos, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}

	kept := Entry{Id: "1", Name: "Renamed", Filename: "kept.jpg", Disabled: true}
	if err := write(Entries{kept, {Id: "2", Name: "gone.jpg", Filename: "gone.jpg"}}); err != nil {
		t.Fatal(err)
	}

	if err := reindex(photos); err != nil {
		t.Fatal(err)
	}

	entries, err := read()
	if err != nil {
		t.Fatal(err)
	}

	byFilename := map[string]Entry{}
	for _, e := range entries {
		byFilename[e.Filename] = e
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(entries), entries)
	}
	if got := byFilename["kept.jpg"]; got != kept {
		t.Errorf("expected the existing entry to be kept as %v, got %v", kept, got)
	}
	if got, ok := byFilename["new.jpg"]; !ok || got.Name != "new.jpg" || got.Id == "" {
		t.Errorf("expected a new entry for new.jpg, got %v", got)
	}
	if _, ok := byFilename["gone.jpg"]; ok {
		t.Error("expected the entry for the missing photo to be removed")
	}
}

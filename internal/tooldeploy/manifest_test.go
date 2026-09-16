package tooldeploy

import (
	"encoding/json"
	"testing"
)

func TestManifest_UpsertAddsThenReplaces(t *testing.T) {
	var m Manifest
	m.upsert(ManifestEntry{Name: "read_file", Version: "1.0.0", ExecutablePath: "a"})
	m.upsert(ManifestEntry{Name: "write_file", Version: "1.0.0", ExecutablePath: "b"})
	if len(m.Tools) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m.Tools))
	}

	m.upsert(ManifestEntry{Name: "read_file", Version: "2.0.0", ExecutablePath: "c"})
	if len(m.Tools) != 2 {
		t.Fatalf("expected upsert of an existing name to replace, not add — got %d entries", len(m.Tools))
	}
	entry, ok := m.find("read_file")
	if !ok || entry.Version != "2.0.0" || entry.ExecutablePath != "c" {
		t.Fatalf("expected read_file updated to version 2.0.0, got %+v", entry)
	}
}

func TestManifest_FindMissingReturnsFalse(t *testing.T) {
	var m Manifest
	if _, ok := m.find("nope"); ok {
		t.Fatal("expected find on an empty manifest to return false")
	}
}

func TestManifest_JSONRoundTrip(t *testing.T) {
	m := Manifest{Tools: []ManifestEntry{
		{Name: "read_file", Description: "d", ExecutablePath: ".vohu/tools/read_file/1.0.0/read_file", Version: "1.0.0"},
	}}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded Manifest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(decoded.Tools) != 1 || decoded.Tools[0] != m.Tools[0] {
		t.Fatalf("round trip mismatch: got %+v", decoded)
	}
}

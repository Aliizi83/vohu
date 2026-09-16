package tooldeploy

import (
	"encoding/json"
	"io"
	"os"
	"path"

	"github.com/pkg/sftp"
)

const manifestPath = ".vohu/tools.json"

type ManifestEntry struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	ExecutablePath string `json:"executable_path"`
	Version        string `json:"version"`
}

type Manifest struct {
	Tools []ManifestEntry `json:"tools"`
}

func (m *Manifest) find(name string) (ManifestEntry, bool) {
	for _, e := range m.Tools {
		if e.Name == name {
			return e, true
		}
	}
	return ManifestEntry{}, false
}

func (m *Manifest) upsert(entry ManifestEntry) {
	for i, e := range m.Tools {
		if e.Name == entry.Name {
			m.Tools[i] = entry
			return
		}
	}
	m.Tools = append(m.Tools, entry)
}

// readManifest returns an empty Manifest, not an error, when the file
// doesn't exist yet — that's just "nothing deployed here so far."
func readManifest(client *sftp.Client) (Manifest, error) {
	f, err := client.Open(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, nil
		}
		return Manifest{}, err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return Manifest{}, err
	}
	if len(data) == 0 {
		return Manifest{}, nil
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func writeManifest(client *sftp.Client, m Manifest) error {
	if err := client.MkdirAll(path.Dir(manifestPath)); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	f, err := client.Create(manifestPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

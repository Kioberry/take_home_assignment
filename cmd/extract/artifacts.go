package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type ArtifactStore struct {
	root string
}

func NewArtifactStore(root, runID string) (*ArtifactStore, error) {
	if root == "" || runID == "" || filepath.Base(runID) != runID || runID == "." || runID == ".." {
		return nil, fmt.Errorf("invalid artifact run id %q", runID)
	}
	dir := filepath.Join(root, runID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create artifact directory %s: %w", dir, err)
	}
	return &ArtifactStore{root: dir}, nil
}

func (s *ArtifactStore) Root() string { return s.root }

func (s *ArtifactStore) artifactPath(name string) (string, error) {
	if name == "" || filepath.Base(name) != name || name == "." || name == ".." {
		return "", fmt.Errorf("invalid artifact name %q", name)
	}
	return filepath.Join(s.root, name), nil
}

func (s *ArtifactStore) WriteJSON(name string, value any) error {
	path, err := s.artifactPath(name)
	if err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open temporary artifact %s: %w", tmpPath, err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		file.Close()
		return fmt.Errorf("encode artifact %s: %w", name, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary artifact %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace artifact %s: %w", path, err)
	}
	return nil
}

func (s *ArtifactStore) ReadJSON(name string, value any) error {
	path, err := s.artifactPath(name)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open artifact %s: %w", path, err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("decode artifact %s: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode artifact %s: multiple JSON values", path)
		}
		return fmt.Errorf("decode artifact %s: %w", path, err)
	}
	return nil
}

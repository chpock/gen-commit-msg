package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	opencode "github.com/sst/opencode-sdk-go"
)

var fixturesDir string

func init() {
	_, f, _, ok := runtime.Caller(0)
	if !ok {
		panic("fixtures: cannot determine source file location")
	}
	fixturesDir = filepath.Join(filepath.Dir(f), "..", "..", "testdata", "fixtures")
}

// SetFixturesDir overrides the directory from which fixtures are loaded.
func SetFixturesDir(dir string) {
	fixturesDir = dir
}

func loadFixtureJSON(name string) ([]byte, error) {
	path := filepath.Join(fixturesDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fixture %s: %w", name, err)
	}
	return data, nil
}

func loadSessionFixture(name string) (*opencode.Session, error) {
	data, err := loadFixtureJSON(name)
	if err != nil {
		return nil, err
	}
	var s opencode.Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unmarshal session fixture %s: %w", name, err)
	}
	return &s, nil
}

func loadPromptFixture(name string) (*opencode.SessionPromptResponse, error) {
	data, err := loadFixtureJSON(name)
	if err != nil {
		return nil, err
	}
	var r opencode.SessionPromptResponse
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("unmarshal prompt fixture %s: %w", name, err)
	}
	return &r, nil
}

func loadDeleteFixture(name string) (bool, error) {
	data, err := loadFixtureJSON(name)
	if err != nil {
		return false, err
	}
	var v bool
	if err := json.Unmarshal(data, &v); err != nil {
		return false, fmt.Errorf("unmarshal delete fixture %s: %w", name, err)
	}
	return v, nil
}

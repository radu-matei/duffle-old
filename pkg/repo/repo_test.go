package repo

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	testfile          = "testdata/local-index.json"
	unorderedTestfile = "testdata/local-index-unordered.json"
	testRepo          = "test-repo"
)

func TestGenerateFromDirectory(t *testing.T) {
	dir := "testdata/repository"
	if err := GenerateFromDirectory(dir, "http://localhost:8080"); err != nil {
		t.Error(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, "repositories")); err != nil {
		t.Error(err)
	}

	// TODO: more pervasive tests.
}

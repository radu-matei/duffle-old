package repo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/deis/duffle/pkg/loader"
	"k8s.io/helm/pkg/provenance"
	"k8s.io/helm/pkg/urlutil"
)

const (
	// APIVersionV1 is the v1 API version for index and repository files.
	APIVersionV1 = "v1"
)

// Maintainer describes a bundle maintainer.
type Maintainer struct {
	// Name is a user name or organization name
	Name string `json:"name,omitempty"`
	// Email is an optional email address to contact the named maintainer
	Email string `json:"email,omitempty"`
	// URL is an optional URL to an address for the named maintainer
	URL string `json:"url,omitempty"`
}

// BundleEntry describes a bundle in the repository.
type BundleEntry struct {
	// The name of the bundle.
	Name string `json:"name"`
	// The version of the bundle.
	Version string `json:"version"`
	// The URL to a relevant project page, git repo, or contact person.
	Home string `json:"home,omitempty"`
	// URLs is a mirror list of URLs to the source code of this bundle.
	URLs []string `json:"urls,omitempty"`
	// A one-sentence description of the bundle.
	Description string `json:"description,omitempty"`
	// A list of string keywords used for searching.
	Keywords []string `json:"keywords,omitempty"`
	// A list of name and URL/email address combinations for the maintainer(s).
	Maintainers []*Maintainer `json:"maintainers,omitempty"`
	// The API Version of this bundle.
	APIVersion string `json:"apiVersion,omitempty"`
	// The shasum digest of the bundle.
	Digest string `json:"digest,omitempty"`
	// The time this entry was added to the index.
	Added time.Time `json:"created,omitempty"`
}

// GenerateFromDirectory reads a (flat) directory and generates a repository.
//
// It indexes only bundles that have been packaged (*.json).
func GenerateFromDirectory(dir, baseURL string) error {
	bundles, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	moreBundles, err := filepath.Glob(filepath.Join(dir, "**/*.json"))
	if err != nil {
		return err
	}
	bundles = append(bundles, moreBundles...)

	for _, bundleFile := range bundles {
		l, err := loader.New(bundleFile)
		if err != nil {
			return err
		}
		b, err := l.Load()
		if err != nil {
			return err
		}

		entry := BundleEntry{
			Name:       b.Name,
			Version:    b.Version,
			APIVersion: APIVersionV1,
		}

		fname, err := filepath.Rel(dir, bundleFile)
		if err != nil {
			return err
		}
		var parentDir string
		parentDir, fname = filepath.Split(fname)
		parentURL, err := urlutil.URLJoin(baseURL, parentDir)
		if err != nil {
			parentURL = filepath.Join(baseURL, parentDir)
		}

		hash, err := provenance.DigestFile(bundleFile)
		if err != nil {
			return err
		}

		fmt.Println("adding", entry.Name, entry.Version)

		u := fname
		if parentURL != "" {
			var err error
			_, file := filepath.Split(fname)
			u, err = urlutil.URLJoin(parentURL, file)
			if err != nil {
				u = filepath.Join(parentURL, file)
			}
		}
		entry.URLs = []string{u}
		entry.Digest = hash
		entry.Added = time.Now()

		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		tagDir := filepath.Join(dir, "repositories", entry.Name, "tags")
		if err := os.MkdirAll(tagDir, 0755); err != nil {
			return err
		}
		if err := ioutil.WriteFile(filepath.Join(tagDir, entry.Version), data, 0644); err != nil {
			return err
		}
	}
	return nil
}

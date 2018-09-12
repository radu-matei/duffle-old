package repo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"time"

	"github.com/Masterminds/semver"
)

const IndexPath = "index.json"

var (
	// ErrNoAPIVersion indicates that an API version was not specified.
	ErrNoAPIVersion = errors.New("no API version specified")
	// ErrNoBundleVersion indicates that a bundle with the given version is not found.
	ErrNoBundleVersion = errors.New("no bundle with the given version found")
	// ErrNoBundleName indicates that a bundle with the given name is not found.
	ErrNoBundleName = errors.New("no bundle name found")
)

// BundleEntries is a list of versioned bundle references.
// Implements a sorter on Version.
type BundleEntries []*BundleEntry

// Len returns the length.
func (c BundleEntries) Len() int { return len(c) }

// Swap swaps the position of two items in the versions slice.
func (c BundleEntries) Swap(i, j int) { c[i], c[j] = c[j], c[i] }

// Less returns true if the version of entry a is less than the version of entry b.
func (c BundleEntries) Less(a, b int) bool {
	// Failed parse pushes to the back.
	i, err := semver.NewVersion(c[a].Version)
	if err != nil {
		return true
	}
	j, err := semver.NewVersion(c[b].Version)
	if err != nil {
		return false
	}
	return i.LessThan(j)
}

// IndexFile represents the index file in a bundle repository
type IndexFile struct {
	APIVersion string                   `json:"apiVersion"`
	Generated  time.Time                `json:"generated"`
	Entries    map[string]BundleEntries `json:"entries"`
	PublicKeys []string                 `json:"publicKeys,omitempty"`
}

// NewIndexFile initializes an index.
func NewIndexFile() *IndexFile {
	return &IndexFile{
		APIVersion: APIVersionV1,
		Generated:  time.Now(),
		Entries:    map[string]BundleEntries{},
		PublicKeys: []string{},
	}
}

// LoadIndexFile takes a file at the given path and returns an IndexFile object
func LoadIndexFile(path string) (*IndexFile, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return loadIndex(b)
}

// Add adds a file to the index
// This can leave the index in an unsorted state
func (i IndexFile) Add(e *BundleEntry) {
	if ee, ok := i.Entries[e.Name]; !ok {
		i.Entries[e.Name] = BundleEntries{e}
	} else {
		i.Entries[e.Name] = append(ee, e)
	}
}

// Has returns true if the index has an entry for a bundle with the given name and exact version.
func (i IndexFile) Has(name, version string) bool {
	_, err := i.Get(name, version)
	return err == nil
}

// SortEntries sorts the entries by version in descending order.
//
// In canonical form, the individual version records should be sorted so that
// the most recent release for every version is in the 0th slot in the
// Entries.BundleEntries array. That way, tooling can predict the newest
// version without needing to parse SemVers.
func (i IndexFile) SortEntries() {
	for _, versions := range i.Entries {
		sort.Sort(sort.Reverse(versions))
	}
}

// Get returns the BundleEntry for the given name.
//
// If version is empty, this will return the bundle with the highest version.
func (i IndexFile) Get(name, version string) (*BundleEntry, error) {
	vs, ok := i.Entries[name]
	if !ok {
		return nil, ErrNoBundleName
	}
	if len(vs) == 0 {
		return nil, ErrNoBundleVersion
	}

	var constraint *semver.Constraints
	if len(version) == 0 {
		constraint, _ = semver.NewConstraint("*")
	} else {
		var err error
		constraint, err = semver.NewConstraint(version)
		if err != nil {
			return nil, err
		}
	}

	for _, ver := range vs {
		test, err := semver.NewVersion(ver.Version)
		if err != nil {
			continue
		}

		if constraint.Check(test) {
			return ver, nil
		}
	}
	return nil, fmt.Errorf("No bundle version found for %s-%s", name, version)
}

// WriteFile writes an index file to the given destination path.
//
// The mode on the file is set to 'mode'.
func (i IndexFile) WriteFile(dest string, mode os.FileMode) error {
	b, err := json.Marshal(i)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dest, b, mode)
}

// Merge merges the given index file into this index.
//
// This merges by name and version.
//
// If one of the entries in the given index does _not_ already exist, it is added.
// In all other cases, the existing record is preserved.
//
// This can leave the index in an unsorted state
func (i *IndexFile) Merge(f *IndexFile) {
	for _, cvs := range f.Entries {
		for _, cv := range cvs {
			if !i.Has(cv.Name, cv.Version) {
				e := i.Entries[cv.Name]
				i.Entries[cv.Name] = append(e, cv)
			}
		}
	}
}

// loadIndex loads an index file and does minimal validity checking.
//
// This will fail if API Version is not set (ErrNoAPIVersion) or if the unmarshal fails.
func loadIndex(data []byte) (*IndexFile, error) {
	i := &IndexFile{}
	if err := json.Unmarshal(data, i); err != nil {
		return i, err
	}
	i.SortEntries()
	return i, nil
}

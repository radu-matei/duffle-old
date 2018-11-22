package store

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/deis/duffle/pkg/signature"

	"github.com/deis/duffle/pkg/bundle"
	"github.com/deis/duffle/pkg/crypto/digest"
	"github.com/deis/duffle/pkg/duffle/home"
	"github.com/deis/duffle/pkg/repo"
)

// LocalStore represents a local bundle storage
type LocalStore struct {
	Home  home.Home
	Index repo.Index
}

func (ls LocalStore) signAndWriteBundle(bf *bundle.Bundle, insecure bool, signer string) (string, error) {
	var (
		d    string
		data []byte
		err  error
	)

	if insecure {
		data = []byte(fmt.Sprintf("%v", bf))
		d, err = digest.OfBuffer(data)
		if err != nil {
			return "", fmt.Errorf("cannot compute digest from bundle: %v", err)
		}
		data = []byte(fmt.Sprintf("%v", bf))
	} else {
		kr, err := signature.LoadKeyRing(ls.Home.SecretKeyRing())
		if err != nil {
			return "", fmt.Errorf("cannot load keyring: %s", err)
		}
		if kr.Len() == 0 {
			return "", errors.New("no signing keys are present in the keyring")
		}

		// Default to the first key in the ring unless the user specifies otherwise.
		key := kr.Keys()[0]
		if signer != "" {
			key, err = kr.Key(signer)
			if err != nil {
				return "", err
			}
		}

		sign := signature.NewSigner(key)
		data, err = sign.Clearsign(bf)
		data = append(data, '\n')
		if err != nil {
			return "", fmt.Errorf("cannot sign bundle: %s", err)
		}
		d, err = digest.OfBuffer(data)
		if err != nil {
			return "", fmt.Errorf("cannot compute digest from bundle: %v", err)
		}
	}

	return d, ioutil.WriteFile(filepath.Join(ls.Home.Bundles(), d), data, 0644)

}

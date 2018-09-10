package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/deis/duffle/pkg/action"
	"github.com/deis/duffle/pkg/bundle"
	"github.com/deis/duffle/pkg/claim"
	"github.com/deis/duffle/pkg/duffle/home"
	"github.com/deis/duffle/pkg/loader"
	"github.com/deis/duffle/pkg/repo"
	"github.com/docker/distribution/reference"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
)

func newInstallCmd(w io.Writer) *cobra.Command {
	const usage = `Install a CNAB bundle

This installs a CNAB bundle with a specific installation name. Once the install is complete,
this bundle can be referenced by installation name.

Example:
	$ duffle install my_release bundles.f1sh.ca/duffle/example:0.1.0
	$ duffle status my_release

Different drivers are available for executing the duffle invocation image. The following drivers
are built-in:

	- docker: run the Docker client. Works for OCI and Docker images
	- debug: fake a run of the invocation image, and print out what would have been sent

Some drivers have additional configuration that can be passed via environment variable.

	docker:
	  - VERBOSE: "true" turns on extra output

UNIX Example:
	$ VERBOSE=true duffle install -d docker my_release https://bundles.f1sh.ca/duffle/example:0.1.0

Windows Example:
	$ $env:VERBOSE = true
	$ duffle install -d docker my_release https://bundles.f1sh.ca/duffle/example:0.1.0

For unpublished CNAB bundles, you can also load the bundle.json directly:

    $ duffle install dev_bundle -f path/to/bundle.json
`
	var (
		installDriver   string
		credentialsFile string
		valuesFile      string
		bundleFile      string

		installationName string
		bundle           bundle.Bundle
	)

	cmd := &cobra.Command{
		Use:   "install NAME BUNDLE",
		Short: "install a CNAB bundle",
		Long:  usage,
		RunE: func(cmd *cobra.Command, args []string) error {
			bundleFile, err := bundleFileOrArg2(args, bundleFile, w)
			if err != nil {
				return err
			}
			installationName = args[0]

			bundle, err = loadBundle(bundleFile)
			if err != nil {
				return err
			}

			if err = validateImage(bundle.InvocationImage); err != nil {
				return err
			}

			driverImpl, err := prepareDriver(installDriver)
			if err != nil {
				return err
			}

			creds, err := loadCredentials(credentialsFile)
			if err != nil {
				return err
			}

			// Because this is an install, we create a new claim. For upgrades, we'd
			// load the claim based on installationName
			c, err := claim.New(installationName)
			if err != nil {
				return err
			}
			c.Bundle = bundle.InvocationImage.Image
			c.ImageType = bundle.InvocationImage.ImageType
			if valuesFile != "" {
				vals, err := parseValues(valuesFile)
				if err != nil {
					return err
				}
				c.Parameters = vals
			}

			inst := &action.Install{
				Driver: driverImpl,
			}
			fmt.Println("Executing install action...")
			err = inst.Run(c, creds)

			// Even if the action fails, we want to store a claim. This is because
			// we cannot know, based on a failure, whether or not any resources were
			// created. So we want to suggest that the user take investigative action.
			err2 := claimStorage().Store(*c)
			if err != nil {
				return fmt.Errorf("Install step failed: %v", err)
			}
			return err2
		},
	}

	cmd.Flags().StringVarP(&credentialsFile, "credentials", "c", "", "Specify a set of credentials to use inside the CNAB bundle")
	cmd.Flags().StringVarP(&installDriver, "driver", "d", "docker", "Specify a driver name")
	cmd.Flags().StringVarP(&valuesFile, "parameters", "p", "", "Specify file containing parameters. Formats: toml, MORE SOON")
	cmd.Flags().StringVarP(&bundleFile, "file", "f", "", "bundle file to install")
	return cmd
}

func bundleFileOrArg2(args []string, bundleFile string, w io.Writer) (string, error) {
	switch {
	case len(args) < 1:
		return "", errors.New("This command requires at least one argument: NAME (name for the installation). It also requires a BUNDLE (CNAB bundle name) or file (using -f)\nValid inputs:\n\t$ duffle install NAME BUNDLE\n\t$ duffle install NAME -f path-to-bundle.json")
	case len(args) == 2 && bundleFile != "":
		return "", errors.New("please use either -f or specify a BUNDLE, but not both")
	case len(args) < 2 && bundleFile == "":
		return "", errors.New("required arguments are NAME (name of the installation) and BUNDLE (CNAB bundle name) or file")
	case len(args) == 2:
		return getBundleFile(args[1])
	}
	return bundleFile, nil
}

func validateImage(img bundle.InvocationImage) error {
	switch img.ImageType {
	case "docker", "oci":
		return validateDockerish(img.Image)
	default:
		return nil
	}
}

func validateDockerish(s string) error {
	if !strings.Contains(s, ":") {
		return errors.New("version is required")
	}
	return nil
}

func parseValues(file string) (map[string]interface{}, error) {
	vals := map[string]interface{}{}
	ext := filepath.Ext(file)
	switch ext {
	case ".toml":
		data, err := ioutil.ReadFile(file)
		if err != nil {
			return vals, err
		}
		err = toml.Unmarshal(data, &vals)
		return vals, err
	case ".json":
		data, err := ioutil.ReadFile(file)
		if err != nil {
			return vals, err
		}
		err = json.Unmarshal(data, &vals)
		return vals, err
	default:
		return vals, errors.New("no decoder for " + ext)
	}
}

func getBundleFile(bundleName string) (string, error) {
	var (
		name  string
		ref   reference.NamedTagged
		proto string
		// repo  string
		// tag   string
	)
	home := home.Home(homePath())

	parts := strings.SplitN(bundleName, "://", 2)
	if len(parts) == 2 {
		proto = parts[0]
		name = parts[1]
	} else {
		proto = "https"
		name = parts[0]
	}
	normalizedRef, err := reference.ParseNormalizedNamed(name)
	if err != nil {
		return "", fmt.Errorf("failed to parse image name: %s: %v", name, err)
	}
	if reference.IsNameOnly(normalizedRef) {
		ref, err = reference.WithTag(normalizedRef, "latest")
		if err != nil {
			// Default tag must be valid, to create a NamedTagged
			// type with non-validated input the WithTag function
			// should be used instead
			panic(err)
		}
	} else {
		if taggedRef, ok := normalizedRef.(reference.NamedTagged); ok {
			ref = taggedRef
		} else {
			return "", fmt.Errorf("unsupported image name: %s", normalizedRef.String())
		}
	}

	// ok, now that we have the name, tag and proto, let's fetch it!
	domain := reference.Domain(ref)
	if domain == "" {
		domain = home.DefaultRepository()
	}

	url := fmt.Sprintf("%s://%s/repositories/%s/tags/%s", proto, domain, reference.Path(ref), ref.Tag())
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request to %s responded with a non-200 status code: %d", url, resp.StatusCode)
	}

	var entry *repo.BundleEntry
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		return "", err
	}

	for _, url := range entry.URLs {
		resp, err := http.Get(url)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("request to %s responded with a non-200 status code: %d", url, resp.StatusCode)
			continue
		}
		bundle, err := bundle.ParseReader(resp.Body)
		if err != nil {
			return "", err
		}
		bundleFilepath := filepath.Join(home.Cache(), fmt.Sprintf("%s-%s.json", bundle.Name, bundle.Version))
		if err := bundle.WriteFile(bundleFilepath, 0644); err != nil {
			return "", err
		}

		return bundleFilepath, nil
	}
	return "", fmt.Errorf("unable to fetch %s %s: no requests to the following URLs succeeded: %v", entry.Name, entry.Version, entry.URLs)
}

func isBundle(filePath string, f os.FileInfo) bool {
	return !f.IsDir() &&
		strings.HasSuffix(f.Name(), ".json") &&
		filepath.Base(filepath.Dir(filePath)) == "bundles"
}

func loadBundle(bundleFile string) (bundle.Bundle, error) {
	l, err := loader.New(bundleFile)
	if err != nil {
		return bundle.Bundle{}, err
	}

	return l.Load()
}

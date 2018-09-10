package main

import (
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/deis/duffle/pkg/duffle/home"
)

func TestGetBundleFile(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	duffleHome = filepath.Join(cwd, "..", "..", "tests", "testdata", "home")
	testHome := home.Home(duffleHome)

	filePath, err := getBundleFile("bundles.f1sh.ca/helloazure:0.1.0")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(filePath)

	expectedFilepath := filepath.Join(testHome.Cache(), "helloazure-0.1.0.json")

	if filePath != expectedFilepath {
		t.Errorf("got '%v', wanted '%v'", filePath, expectedFilepath)
	}
}

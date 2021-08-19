package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/circle-makotom/orbs-sync/types"
	"github.com/pkg/errors"
)

// https://github.com/CircleCI-Public/circleci-cli/blob/6ec121d68a6b12f46c604cc0f44d1e18d8bb2b52/cmd/root.go#L16
const APIEndpoint = "graphql-unstable"

var knownHiddenOrbs = []string{"circleci/welcome-orb", "circleci/artifactory", "circleci/hello-build"}

func getSafeOrbSrcFileName(orbRef string) string {
	return fmt.Sprintf("%s.yml", url.QueryEscape(orbRef))
}

func loadOrbYAML(orbRef, srcFilePath string) (*types.VersionedOrb, error) {
	logger := log.New(os.Stderr, "load-orb-yaml: ", 7)

	orbRefParts := strings.Split(orbRef, "@")
	orbName := orbRefParts[0]
	orbVersion := strings.Join(orbRefParts[1:], "@")

	logger.Printf("loading %q", orbRef)

	orbSrc, err := ioutil.ReadFile(srcFilePath)
	if err != nil {
		return nil, err
	}

	return &types.VersionedOrb{
		Ref:     orbRef,
		Name:    orbName,
		Version: orbVersion,
		Source:  string(orbSrc),
	}, nil
}

func loadOrbsInDir(orbSrcDirPath string) ([]*types.VersionedOrb, error) {
	ret := []*types.VersionedOrb{}

	orbSrcFiles, err := ioutil.ReadDir(orbSrcDirPath)
	if err != nil {
		return nil, err
	}

	for _, fileInfo := range orbSrcFiles {
		if !fileInfo.IsDir() {
			srcFilePath := fileInfo.Name()

			orbRef, err := url.QueryUnescape(regexp.MustCompile(`(?i)\.yml$`).ReplaceAllString(srcFilePath, ""))
			if err != nil {
				return nil, errors.Wrapf(err, "failed to decode file name %q for orb ref", srcFilePath)
			}

			orb, err := loadOrbYAML(orbRef, path.Join(orbSrcDirPath, srcFilePath))
			if err != nil {
				return nil, errors.Wrapf(err, "could not load orb %q", orbRef)
			}

			ret = append(ret, orb)
		}
	}

	return ret, nil
}

func loadListedOrbs(orderedListPath, orbSrcDirPath string) ([]*types.VersionedOrb, error) {
	ret := []*types.VersionedOrb{}

	orderedListStr, err := ioutil.ReadFile(orderedListPath)
	if err != nil {
		return nil, err
	}
	orderedList := strings.Split(strings.TrimSpace(string(orderedListStr)), "\n")

	for _, orbRef := range orderedList {
		orb, err := loadOrbYAML(orbRef, path.Join(orbSrcDirPath, getSafeOrbSrcFileName(orbRef)))
		if err != nil {
			return nil, errors.Wrapf(err, "could not load orb %q", orbRef)
		}

		ret = append(ret, orb)
	}

	return ret, nil
}

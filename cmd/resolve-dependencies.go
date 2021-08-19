package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	depresolver "github.com/circle-makotom/orbs-sync/dependency-resolver"
	"github.com/circle-makotom/orbs-sync/types"
)

type ResolveDependenciesOpts struct {
	OrbSrcDirPath     string
	OrderedListPath   string
	IllegibleListPath string
	UnresolvedMapPath string
}

func cmdResolveDependencies() *cobra.Command {
	opts := &ResolveDependenciesOpts{}

	cmd := &cobra.Command{
		Use:   "resolve-dependencies",
		Short: "Resolve dependencies between orbs and return the order of orbs to import",
		RunE: func(_ *cobra.Command, _ []string) error {
			return ResolveDependencies(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.OrbSrcDirPath, "src", "orbs", "Path to the directory containing orb sources")
	flags.StringVar(&opts.OrderedListPath, "ordered", "orbs-resolved.txt", "Path to the file to list resolved/ordered orbs")
	flags.StringVar(&opts.IllegibleListPath, "illegible", "orbs-illegible.txt", "Path to the file to dump the list of orbs caused YAML parser errors")
	flags.StringVar(&opts.UnresolvedMapPath, "unresolved", "orbs-unresolved.txt", "Path to the file to dump the map of unresolved orbs")

	return cmd
}

func dumpResolvedOrbs(filename string, resolvedOrder []*types.VersionedOrb) error {
	contents := []string{}

	for _, orb := range resolvedOrder {
		contents = append(contents, orb.Ref)
	}

	return ioutil.WriteFile(filename, []byte(strings.Join(contents, "\n")), 0644)
}

func dumpIllegibleOrbs(filename string, illegibleOrbRefs []string) error {
	return ioutil.WriteFile(filename, []byte(strings.Join(illegibleOrbRefs, "\n")), 0644)
}

func formatUnresolvedMap(unresolvedMap map[string][]string) string {
	contents := []string{}

	for orbRef, dependencies := range unresolvedMap {
		depQuoted := []string{}

		for _, dependencyTarget := range dependencies {
			depQuoted = append(depQuoted, fmt.Sprintf("%q", dependencyTarget))
		}

		contents = append(contents, fmt.Sprintf("%q => [ %s ]", orbRef, strings.Join(depQuoted, " ")))
	}

	return strings.Join(contents, "\n")
}

func dumpUnresolvedOrbs(filename string, unresolvedMap map[string][]string) error {
	return ioutil.WriteFile(filename, []byte(formatUnresolvedMap(unresolvedMap)), 0644)
}

func ResolveDependencies(opts *ResolveDependenciesOpts) error {
	logger := log.New(os.Stderr, "resolve-dependencies: ", 7)

	// Load orbs
	logger.Printf("loading orbs")
	orbs, err := loadOrbsInDir(opts.OrbSrcDirPath)
	if err != nil {
		return errors.Wrap(err, "could not load orbs")
	}

	// Resolve dependencies
	logger.Printf("resolving dependencies")
	resolvedOrder, illegible, unresolved, err := depresolver.Resolve(orbs)
	if err != nil {
		return errors.Wrap(err, "dependency resolver failed")
	}

	// Dump results
	if err := dumpResolvedOrbs(opts.OrderedListPath, resolvedOrder); err != nil {
		return errors.Wrap(err, "could not dump the list of resolved orbs")
	}
	if err := dumpIllegibleOrbs(opts.IllegibleListPath, illegible); err != nil {
		return errors.Wrap(err, "could not dump the list of orbs caused YAML parser errors")
	}
	if err := dumpUnresolvedOrbs(opts.UnresolvedMapPath, unresolved); err != nil {
		return errors.Wrap(err, "could not dump the map of unresolved orbs")
	}

	return nil
}

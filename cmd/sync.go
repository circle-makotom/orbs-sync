package cmd

import (
	"log"
	"os"
	"strings"

	bulkimporter "github.com/circle-makotom/orbs-sync/bulk-importer"
	"github.com/circle-makotom/orbs-sync/collector"
	depresolver "github.com/circle-makotom/orbs-sync/dependency-resolver"
	"github.com/circle-makotom/orbs-sync/types"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type SyncOpts struct {
	SrcHostname        string
	SrcToken           string
	DstHostname        string
	DstToken           string
	BeSlow             bool
	IncludeUncertified bool
	KnownHiddenOrbs    []string
}

func cmdSync() *cobra.Command {
	opts := &SyncOpts{}

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync orbs",
		RunE: func(_ *cobra.Command, _ []string) error {
			return Sync(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.SrcHostname, "src-host", "https://circleci.com", "Hostname of the CircleCI instance from where orbs are coming")
	flags.StringVar(&opts.SrcToken, "src-token", "", "Token for the CircleCI instance from where orbs are coming")
	flags.StringVar(&opts.DstHostname, "dst-host", "", "Hostname of the CircleCI instance to where orbs are going")
	flags.StringVar(&opts.DstToken, "dst-token", "", "Token for the CircleCI instance to where orbs are going")
	flags.BoolVar(&opts.BeSlow, "slow", false, "Use slower strategy to avoid errors on big orbs")
	flags.BoolVar(&opts.IncludeUncertified, "include-uncertified", false, "Fetch uncertified orbs as well")
	flags.StringSliceVar(&opts.KnownHiddenOrbs, "must-include", knownHiddenOrbs, "Orbs to be included regardlessly - used for well-known hidden orbs")

	cmd.MarkFlagRequired("src-token")
	cmd.MarkFlagRequired("dst-host")
	cmd.MarkFlagRequired("dst-token")

	return cmd
}

func copyOrbsExcept(original, except []*types.VersionedOrb) []*types.VersionedOrb {
	ret := []*types.VersionedOrb{}

	isInException := make(map[string]bool)

	for _, exceptEntry := range except {
		isInException[exceptEntry.Ref] = true
	}

	for _, originalEntry := range original {
		if !isInException[originalEntry.Ref] {
			ret = append(ret, originalEntry)
		}
	}

	return ret
}

func Sync(opts *SyncOpts) error {
	logger := log.New(os.Stderr, "sync: ", 7)

	// Fetch orbs from src
	srcOrbs, err := collector.ListAllVersionedOrbsWithNewClient(opts.SrcHostname, APIEndpoint, opts.SrcToken, opts.KnownHiddenOrbs, true, opts.IncludeUncertified, opts.BeSlow, debug)
	if err != nil {
		return errors.Wrap(err, "could not fetch orbs from source")
	}

	// List orbs on dst
	dstOrbs, err := collector.ListAllVersionedOrbsWithNewClient(opts.DstHostname, APIEndpoint, opts.DstToken, opts.KnownHiddenOrbs, false, opts.IncludeUncertified, opts.BeSlow, debug)
	if err != nil {
		return errors.Wrap(err, "could not list orbs on destination")
	}

	// Resolve dependencies
	orbsInResolvedOrder, illegible, unresolved, err := depresolver.Resolve(srcOrbs)
	if err != nil {
		return errors.Wrap(err, "dependency resolver failed")
	}

	// Filter those already available on destination
	filteredOrbsInResolvedOrder := copyOrbsExcept(orbsInResolvedOrder, dstOrbs)

	// Import orbs
	_, dropped, err := bulkimporter.ImportOrbsWithNewClient(filteredOrbsInResolvedOrder, opts.DstHostname, APIEndpoint, opts.DstToken, debug)
	if err != nil {
		return errors.Wrap(err, "import failed")
	}

	logger.Printf("here is the list of orbs caused YAML parser error\n\n%v\n\n", strings.Join(illegible, "\n"))
	logger.Printf("here is the map of orbs with unresolvable dependencies\n\n%v\n\n", formatUnresolvedMap(unresolved))
	logger.Printf("here is the list of orbs dropped during import\n\n%v\n\n", strings.Join(dropped, "\n"))

	logger.Println("sync completed!")

	return nil
}

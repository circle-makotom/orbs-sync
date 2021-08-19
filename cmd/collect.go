package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/circle-makotom/orbs-sync/collector"
)

type CollectOpts struct {
	Hostname           string
	Token              string
	ListPath           string
	SrcDirPath         string
	ListOnly           bool
	BeSlow             bool
	IncludeUncertified bool
	KnownHiddenOrbs    []string
}

func cmdCollect() *cobra.Command {
	opts := &CollectOpts{}

	cmd := &cobra.Command{
		Use:   "collect",
		Short: "List and fetch orbs",
		RunE: func(_ *cobra.Command, _ []string) error {
			return CollectOrbs(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.Hostname, "host", "https://circleci.com", "Hostname of the CircleCI instance to communicate with")
	flags.StringVar(&opts.Token, "token", "", "Token for the CircleCI instance to communicate with")
	flags.StringVar(&opts.ListPath, "list", "orbs.txt", "Path to the file to put the list of orbs")
	flags.StringVar(&opts.SrcDirPath, "src", "orbs", "Path to the directory to put fetched orb sources")
	flags.BoolVar(&opts.ListOnly, "list-only", false, "Do not fetch orb sources; just list names and versions")
	flags.BoolVar(&opts.BeSlow, "slow", false, "Use slower strategy to avoid errors on big orbs")
	flags.BoolVar(&opts.IncludeUncertified, "include-uncertified", false, "Fetch uncertified orbs as well")
	flags.StringSliceVar(&opts.KnownHiddenOrbs, "must-include", knownHiddenOrbs, "Orbs to be included regardlessly - used for well-known hidden orbs")

	cmd.MarkFlagRequired("token")

	return cmd
}

func CollectOrbs(opts *CollectOpts) error {
	logger := log.New(os.Stderr, "collect: ", 7)

	logger.Printf("start collecting orbs")

	// Fetch orbs
	orbs, err := collector.ListAllVersionedOrbsWithNewClient(opts.Hostname, APIEndpoint, opts.Token, opts.KnownHiddenOrbs, !opts.ListOnly, opts.IncludeUncertified, opts.BeSlow, debug)
	if err != nil {
		return errors.Wrap(err, "could not fetch orbs")
	}

	logger.Printf("collection done; proceeding to outputting")

	// Create a file to put the list of orbs
	listFile, err := os.Create(opts.ListPath)
	if err != nil {
		return errors.Wrap(err, "could not create a file for the list of orbs")
	}

	if !opts.ListOnly {
		// Create a directory to put orb sources
		if err := os.Mkdir(opts.SrcDirPath, 0755); err != nil && !errors.Is(err, os.ErrExist) {
			return errors.Wrap(err, "could not create a directory for orb sources")
		}
	}

	// Walk through each orb
	logger.Printf("walking through each orb")
	for _, orb := range orbs {
		logger.Printf("processing %q", orb.Ref)

		_, err := listFile.Write([]byte(fmt.Sprintf("%s\n", orb.Ref)))
		if err != nil {
			return errors.Wrapf(err, "failed to add %q to the list of orbs", orb.Ref)
		}

		// Dump orb sources unless requested not to
		if !opts.ListOnly {
			if err := ioutil.WriteFile(path.Join(opts.SrcDirPath, getSafeOrbSrcFileName(orb.Ref)), []byte(orb.Source), 0644); err != nil {
				return errors.Wrapf(err, "failed to dump the source of %q", orb.Ref)
			}
		}
	}

	return nil
}

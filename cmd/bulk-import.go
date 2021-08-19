package cmd

import (
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	bulkimporter "github.com/circle-makotom/orbs-sync/bulk-importer"
)

type BulkImportOpts struct {
	Hostname          string
	Token             string
	OrderedListPath   string
	OrbSrcDirPath     string
	AvailableListPath string
	DroppedListPath   string
}

func cmdBulkImport() *cobra.Command {
	opts := &BulkImportOpts{}

	cmd := &cobra.Command{
		Use:   "bulk-import",
		Short: "Import multiple orbs at once",
		RunE: func(_ *cobra.Command, _ []string) error {
			return BulkImport(opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.Hostname, "host", "", "Hostname of the CircleCI instance to communicate with")
	flags.StringVar(&opts.Token, "token", "", "Token for the CircleCI instance to communicate with")
	flags.StringVar(&opts.OrderedListPath, "list", "orbs-resolved.txt", "Path to the file containing the list of resolved/ordered orbs")
	flags.StringVar(&opts.OrbSrcDirPath, "src", "orbs", "Path to the directory containing orb sources")
	flags.StringVar(&opts.AvailableListPath, "available", "orbs-available.txt", "Path to the file to put the list of orbs ensured to be available by import")
	flags.StringVar(&opts.DroppedListPath, "dropped", "orbs-dropped.txt", "Path to the file to put the list of dropped orbs while importing")

	cmd.MarkFlagRequired("host")
	cmd.MarkFlagRequired("token")

	return cmd
}

func dumpProcessedOrbRefs(availableListPath, droppedListPath string, available, dropped []string) error {
	if err := ioutil.WriteFile(availableListPath, []byte(strings.Join(available, "\n")), 0644); err != nil {
		return errors.Wrap(err, "could not dump available orbs")
	}
	if err := ioutil.WriteFile(droppedListPath, []byte(strings.Join(dropped, "\n")), 0644); err != nil {
		return errors.Wrap(err, "could not dump dropped orbs")
	}

	return nil
}

func BulkImport(opts *BulkImportOpts) error {
	logger := log.New(os.Stderr, "bulk-import: ", 7)

	// Load orbs
	logger.Printf("loading orbs")
	orbs, err := loadListedOrbs(opts.OrderedListPath, opts.OrbSrcDirPath)
	if err != nil {
		return errors.Wrap(err, "could not load orbs")
	}

	// Import orbs
	logger.Printf("starting import")
	available, dropped, err := bulkimporter.ImportOrbsWithNewClient(orbs, opts.Hostname, APIEndpoint, opts.Token, debug)
	if err != nil {
		return errors.Wrap(err, "import failed")
	}

	// Dump available/dropped orbs
	logger.Printf("outputting results")
	if err := dumpProcessedOrbRefs(opts.AvailableListPath, opts.DroppedListPath, available, dropped); err != nil {
		return errors.Wrap(err, "could not dump the lists of processed orbs")
	}

	return nil
}

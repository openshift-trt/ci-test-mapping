package cmd

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/openshift-eng/ci-test-mapping/cmd/ci-test-mapping/flags"
	v1 "github.com/openshift-eng/ci-test-mapping/pkg/api/types/v1"
	"github.com/openshift-eng/ci-test-mapping/pkg/bigquery"
)

var pruneCommand = &cobra.Command{
	Use:   "prune",
	Short: "Prune older mapping records from the database",
	Run: func(cmd *cobra.Command, args []string) {
		// Get a bigquery client
		bigqueryClient, err := bigquery.NewClient(context.Background(),
			pruneFlags.bigqueryFlags.ServiceAccountCredentialFile,
			pruneFlags.bigqueryFlags.OAuthClientCredentialFile, pruneFlags.bigqueryFlags.Project, pruneFlags.bigqueryFlags.Dataset)
		if err != nil {
			log.WithError(err).Fatal("could not obtain bigquery client")
			cmd.Usage() //nolint
		}

		// Create or update schema for mapping table
		testTableManager := bigquery.NewMappingTableManager[v1.TestOwnership](context.Background(), bigqueryClient, pruneFlags.mappingTable, v1.TestMappingTableSchema)
		if err := testTableManager.PruneMappings(); err != nil {
			log.WithError(err).Fatal("could not prune test mapping table")
		}

		// Create or update schema for mapping table
		variantTableManager := bigquery.NewMappingTableManager[v1.VariantMapping](context.Background(), bigqueryClient, pruneFlags.variantMappingTable, v1.VariantMappingTableSchema)
		if err := variantTableManager.PruneMappings(); err != nil {
			log.WithError(err).Fatal("could not prune variant mapping table")
		}

	},
}

type PruneFlags struct {
	bigqueryFlags       *flags.BigQueryFlags
	mappingTable        string
	variantMappingTable string
}

var pruneFlags = NewPruneFlags()

func NewPruneFlags() *PruneFlags {
	return &PruneFlags{
		bigqueryFlags: flags.NewBigQueryFlags(),
	}
}

func (f *PruneFlags) BindFlags(fs *pflag.FlagSet) {
	pruneFlags.bigqueryFlags.BindFlags(fs)
}

func init() {
	pruneCommand.PersistentFlags().StringVar(&pruneFlags.mappingTable, "table-mapping", "component_mapping", "BigQuery table name storing component mappings")
	pruneCommand.PersistentFlags().StringVar(&pruneFlags.variantMappingTable, "variant-table-mapping", "variant_mapping", "BigQuery table name storing variant mappings")
	pruneFlags.BindFlags(pruneCommand.Flags())
	rootCmd.AddCommand(pruneCommand)
}

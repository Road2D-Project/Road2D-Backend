package cmd

import (
	"log"
	"os"

	"Road-To-Destination-BE/cmd/cli/seed"
	migraterun "Road-To-Destination-BE/cmd/migrate/run"
	serverrun "Road-To-Destination-BE/cmd/server/run"
	"Road-To-Destination-BE/internal/seed/location"
	"Road-To-Destination-BE/module/share"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "r2d",
	Short:         "Road2D backend CLI",
	Long:          "server runs the API, cli seeds reference data, migrate applies the schema.",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := share.LoadDotEnv(".env"); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	},
}

func init() {
	//Register subcommand to root
	rootCmd.AddCommand(
		&cobra.Command{
			Use:   "server",
			Short: "Start the HTTP API",
			RunE: func(cmd *cobra.Command, args []string) error {
				return serverrun.Run()
			},
		},
		seed.NewSeederCommand(),
		&cobra.Command{
			Use:   "migrate",
			Short: "Apply database schema changes",
			RunE: func(cmd *cobra.Command, args []string) error {
				return migraterun.Run()
			},
		},
	)
}

func Execute() {
	if len(os.Args) > 1 {
		os.Args = append([]string{os.Args[0]}, location.RepairArgs(os.Args[1:])...)
	}
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

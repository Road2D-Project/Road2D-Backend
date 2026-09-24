package seed

import (
	"log"
	"os"

	"Road-To-Destination-BE/module/share"
	"Road-To-Destination-BE/module/share/configuration"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

type deps struct {
	db    *gorm.DB
	redis *redis.Client
}

var runtime deps

func NewSeederCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "seeder",
		Short:         "Insert reference data",
		Long:          "Seed subcommands write verified rows. Add a new file under cmd/cli/seed for each entity.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := share.LoadDotEnv(".env"); err != nil && !os.IsNotExist(err) {
				return err
			}
			var dbConfig configuration.DatabaseConfig
			if err := dbConfig.ConnectDatabase(); err != nil {
				return err
			}
			if err := configuration.AutoMigrate(dbConfig.GetDatabase()); err != nil {
				return err
			}
			var redisConfig configuration.RedisConfiguration
			redisConfig.Connect()
			runtime.db = dbConfig.GetDatabase()
			runtime.redis = redisConfig.Client()
			log.Printf("postgres host=%s db=%s user=%s",
				share.GetEnvStringDefault("DB_HOST", ""),
				share.GetEnvStringDefault("DB_NAME", ""),
				share.GetEnvStringDefault("DB_USER", ""),
			)
			return nil
		},
	}
	// Wire new seeder entity here
	cmd.AddCommand(newLocationCommand())
	cmd.AddCommand(newUserCommand())
	return cmd
}

func Execute(args []string) error {
	cmd := NewSeederCommand()
	cmd.SetArgs(args)
	return cmd.Execute()
}

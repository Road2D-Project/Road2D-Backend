package seed

import (
	"context"
	"errors"
	"log"

	"Road-To-Destination-BE/internal/seed/user"
	"Road-To-Destination-BE/module/authentication/repository"
	"Road-To-Destination-BE/module/authentication/service"

	"github.com/spf13/cobra"
)

func newUserCommand() *cobra.Command {
	var isDefault bool
	var fake bool
	var file string
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Seed users",
		Long: `Register accounts that are not already in the database.

  go run . seeder user --default
  go run . seeder user --fake
  go run . seeder user --fake --file users.json

--fake reads internal/seed/user/users.json. Pass --file to use another JSON array.
The array length is not capped.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUser(isDefault, fake, file)
		},
	}
	cmd.Flags().BoolVarP(&isDefault, "default", "d", false, "Register USER_NAME from the environment and print tokens")
	cmd.Flags().BoolVar(&fake, "fake", false, "Register every user in the fake-user JSON")
	cmd.Flags().StringVar(&file, "file", "", "JSON array of {username, email, password}; default is the embedded users.json")
	cmd.MarkFlagsMutuallyExclusive("default", "fake")
	return cmd
}

func runUser(isDefault bool, fake bool, file string) error {
	if isDefault == fake {
		return errors.New("pass either --default or --fake")
	}
	repo := repository.NewUserRepository(runtime.db)
	if isDefault {
		access, refresh, err := user.LoginDefaultUser(context.Background(), service.NewLoginService(repo), service.NewRegisterService(repo), repo)
		if err != nil {
			return err
		}
		log.Println("Access:", access)
		log.Println("Refresh:", refresh)
		return nil
	}
	users, err := fakeUsers(file)
	if err != nil {
		return err
	}
	report, err := user.SeedFakeUsers(context.Background(), service.NewRegisterService(repo), repo, users)
	if report != nil {
		log.Printf("fake users created=%d skipped=%d", len(report.Created), len(report.Skipped))
		for _, name := range report.Created {
			log.Printf("  created %s", name)
		}
		for _, name := range report.Skipped {
			log.Printf("  skipped %s", name)
		}
	}
	return err
}

func fakeUsers(file string) ([]user.FakeUser, error) {
	if file == "" {
		return user.DefaultFakeUsers()
	}
	return user.LoadFakeUsers(file)
}

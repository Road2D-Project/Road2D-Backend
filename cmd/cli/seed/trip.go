package seed

import (
	"context"
	"log"

	"Road-To-Destination-BE/internal/seed/trip"

	"github.com/spf13/cobra"
)

func newTripCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "trip",
		Short: "Seed a group, a trip, and a route from forked destinations",
		Long: `Create a group and a planning trip owned by the default user, then place up to 10 pins on the main branch.

  go run . seeder trip

USER_NAME becomes the group owner and the trip leader. The user is registered when that username is missing.
Each pin is named from its index and the location name. The route is rebuilt by looking those names up in the same order.
Locations must already exist (seeder location).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := trip.Seed(context.Background(), runtime.db, runtime.redis)
			if report != nil {
				log.Printf("user=%s group=%s trip=%s pins=%d", report.Username, report.GroupID, report.TripID, len(report.Pins))
				for i, pin := range report.Pins {
					log.Printf("  [%d] %s %s", i, pin.Name, pin.ID)
				}
			}
			return err
		},
	}
}

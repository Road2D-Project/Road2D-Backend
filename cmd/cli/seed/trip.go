package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"Road-To-Destination-BE/internal/seed/trip"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func newTripCommand() *cobra.Command {
	var (
		name        string
		note        string
		tripType    string
		public      bool
		members     []string
		locationIDs []string
	)
	cmd := &cobra.Command{
		Use:   "trip",
		Short: "Seed a trip, its members, and a main branch from location ids",
		Long: `Create a planning trip owned by USER_NAME, seat a member list, and place locations on the main branch in the given order.

  go run . seeder trip
  go run . seeder trip --public --type bronze --members baokhoa,camtuyen --location-ids <uuid>,<uuid>

USER_NAME becomes the trip leader and is registered when that username is missing.
With no --members, every account in internal/seed/user/users.json except the leader is seated, and missing accounts are registered first.
With no --location-ids, the earliest locations are used (up to 10). Run seeder location first.
The command prints the trip (no secrets) and the join token.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			parsedType, err := enum.TripTypeString(strings.TrimSpace(tripType))
			if err != nil {
				return fmt.Errorf("type must be bronze, silver, gold, or diamond: %w", err)
			}
			ids := make([]uuid.UUID, 0, len(locationIDs))
			for _, raw := range locationIDs {
				raw = strings.TrimSpace(raw)
				if raw == "" {
					continue
				}
				id, err := uuid.Parse(raw)
				if err != nil {
					return fmt.Errorf("location id %q: %w", raw, err)
				}
				ids = append(ids, id)
			}
			report, err := trip.Seed(context.Background(), runtime.db, runtime.redis, trip.Options{
				Name:            name,
				Note:            note,
				TripType:        parsedType,
				Visibility:      public,
				MemberUsernames: members,
				LocationIDs:     ids,
			})
			if report != nil {
				raw, marshalErr := json.MarshalIndent(report, "", "  ")
				if marshalErr != nil {
					return marshalErr
				}
				fmt.Println(string(raw))
			}
			return err
		},
	}
	cmd.Flags().StringVar(&name, "name", "Seed trip", "trip name")
	cmd.Flags().StringVar(&note, "note", "", "trip note")
	cmd.Flags().StringVar(&tripType, "type", "bronze", "trip type: bronze, silver, gold, or diamond")
	cmd.Flags().BoolVar(&public, "public", false, "visibility true so outsiders can find the trip")
	cmd.Flags().StringSliceVar(&members, "members", nil, "usernames to seat as members, besides the leader")
	cmd.Flags().StringSliceVar(&locationIDs, "location-ids", nil, "catalog location ids for the main branch, in stop order")
	return cmd
}

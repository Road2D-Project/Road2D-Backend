package seed

import (
	"context"
	"errors"
	"fmt"
	"log"

	"Road-To-Destination-BE/internal/seed/location"
	"Road-To-Destination-BE/module/maps/client"
	"Road-To-Destination-BE/module/share"
	"Road-To-Destination-BE/module/trip/repository"

	"github.com/spf13/cobra"
)

func newLocationCommand() *cobra.Command {
	var lat, lng float64
	var coords string
	var limit int

	cmd := &cobra.Command{
		Use:   "location",
		Short: "Reverse-geocode pins into verified locations",
		Long: `Geocode each pin (default limit 10), then persist new Goong places.

Prefer space-separated flags so PowerShell does not split decimals:

  go run . seeder location --lat 10.7486 --lng 106.6601
  go run .\cmd\seeder\main.go location --lat 10.7486 --lng 106.6601
  go run . seeder location --coords "10.7486,106.6601"
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLocation(cmd, args, lat, lng, coords, limit)
		},
	}
	cmd.Flags().Float64Var(&lat, "lat", 0, "pin latitude")
	cmd.Flags().Float64Var(&lng, "lng", 0, "pin longitude")
	cmd.Flags().StringVar(&coords, "coords", "", "lat,lng pairs separated by ;")
	cmd.Flags().IntVar(&limit, "limit", 10, "Goong geocode result cap per pin")
	cmd.MarkFlagsRequiredTogether("lat", "lng")
	return cmd
}

func runLocation(cmd *cobra.Command, args []string, lat, lng float64, coords string, limit int) error {
	args = location.RepairArgs(args)
	if share.GetEnvStringDefault("GOONG_MAP_CALC_API_KEY", "") == "" {
		return errors.New("set GOONG_MAP_CALC_API_KEY")
	}
	pins, err := locationPins(cmd, args, lat, lng, coords)
	if err != nil {
		return err
	}
	for i, pin := range pins {
		log.Printf("pin[%d]=%g,%g", i, pin.Lat, pin.Lng)
	}

	seeder := location.NewSeeder(
		client.NewDefaultGoongClient(),
		repository.NewLocationRepository(runtime.db),
		repository.NewPlaceLocationStore(runtime.redis),
	)
	seeder.Limit = limit
	report, err := seeder.SeedFromCoords(context.Background(), pins)
	if report != nil {
		printLocationReport(report)
	}
	if err != nil {
		if errors.Is(err, client.ErrMissingAPIKey) {
			return errors.New("set GOONG_MAP_CALC_API_KEY")
		}
		return err
	}
	if report.Wrote() == 0 {
		log.Print("no rows written — already in db/cache, empty geocode, or unmapped detail")
	}
	return nil
}

func locationPins(cmd *cobra.Command, args []string, lat, lng float64, coords string) ([]location.Coord, error) {
	tokens := []string{coords}
	if cmd.Flags().Changed("lat") || cmd.Flags().Changed("lng") {
		tokens = append(tokens, fmt.Sprintf("%g", lat), fmt.Sprintf("%g", lng))
	}
	tokens = append(tokens, location.NumericArgs(args)...)
	parsed, err := location.ParseCoordTokens(tokens...)
	if err != nil {
		return nil, err
	}
	if len(parsed) == 0 {
		log.Print("no pins given; using DefaultCoords")
		return location.DefaultCoords(), nil
	}
	return parsed, nil
}

func printLocationReport(report *location.Report) {
	for _, event := range report.Events {
		name := event.Name
		if name == "" {
			name = "-"
		}
		placeID := event.PlaceID
		if len(placeID) > 12 {
			placeID = placeID[:12] + "…"
		}
		if placeID == "" {
			placeID = "-"
		}
		log.Printf("  %s  name=%q place_id=%s", event.Status, name, placeID)
	}
	log.Printf(
		"summary pins=%d geocode_results=%d created=%d from_cache=%d skipped_db=%d skipped_cache=%d skipped_seen=%d skipped_empty=%d unmapped=%d wrote=%d",
		report.Pins,
		report.GeocodeResults,
		report.Created,
		report.Backfilled,
		report.AlreadyInDB,
		report.AlreadyCached,
		report.DuplicateInRun,
		report.SkippedEmpty,
		report.UnmappedDetail,
		report.Wrote(),
	)
}

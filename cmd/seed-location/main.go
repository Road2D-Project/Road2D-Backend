package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"Road-To-Destination-BE/module/maps/client"
	"Road-To-Destination-BE/module/share"
	"Road-To-Destination-BE/module/share/configuration"
	"Road-To-Destination-BE/module/trip/repository"
)

func main() {
	log.SetFlags(log.LstdFlags)
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `Seed verified locations from reverse-geocode pins.

  go run ./cmd/seed-location -lat=10.7486 -lng=106.6601
  go run ./cmd/seed-location --%% 10.7486 106.6601

PowerShell splits 10.7486 at the dot. The CLI stitches those tokens back.
Prefer -lat / -lng. Quote -coords if you use it:

  go run ./cmd/seed-location -coords="10.7486,106.6601"

`)
		flag.PrintDefaults()
	}

	flag.CommandLine.Init("seed-location", flag.ExitOnError)
	coordsFlag := flag.String("coords", "", "lat,lng pairs separated by ; (quote the value in PowerShell)")
	vFlag := flag.String("v", "", "alias for -coords")
	latFlag := flag.String("lat", "", "single pin latitude (use with -lng)")
	lngFlag := flag.String("lng", "", "single pin longitude (use with -lat)")
	repaired := RepairSeedArgs(os.Args[1:])
	if err := flag.CommandLine.Parse(repaired); err != nil {
		log.Fatal(err)
	}

	cwd, _ := os.Getwd()
	log.Printf("cwd=%s", cwd)
	log.Printf("argv repaired=%q", repaired)
	log.Printf("flags coords=%q v=%q lat=%q lng=%q extra=%q", *coordsFlag, *vFlag, *latFlag, *lngFlag, flag.Args())
	if err := share.LoadDotEnv(".env"); err != nil {
		if os.IsNotExist(err) {
			log.Print(".env not found in cwd; using process env")
		} else {
			log.Fatal(err)
		}
	} else {
		log.Print("loaded .env")
	}

	tokens := []string{*coordsFlag, *vFlag}
	if *latFlag != "" || *lngFlag != "" {
		if *latFlag == "" || *lngFlag == "" {
			log.Fatal("need both -lat and -lng")
		}
		tokens = append(tokens, *latFlag, *lngFlag)
	}
	tokens = append(tokens, numericArgs(flag.Args())...)
	coords, err := parseSeedCoordTokens(tokens...)
	if err != nil {
		log.Fatal(err)
	}
	if len(coords) == 0 {
		coords = defaultSeedCoords()
		log.Print("no pins given; using defaultSeedCoords")
	}
	for i, pin := range coords {
		log.Printf("pin[%d]=%g,%g", i, pin.Lat, pin.Lng)
	}

	host := share.GetEnvStringDefault("DB_HOST", "")
	name := share.GetEnvStringDefault("DB_NAME", "")
	user := share.GetEnvStringDefault("DB_USER", "")
	log.Printf("postgres host=%s db=%s user=%s", host, name, user)
	log.Printf("redis=%s", share.GetEnvStringDefault("REDIS_ADDR", "localhost:6379"))
	if share.GetEnvStringDefault("GOONG_MAP_CALC_API_KEY", "") == "" {
		log.Fatal("set GOONG_MAP_CALC_API_KEY")
	}

	var dbConfig configuration.DatabaseConfig
	if err := dbConfig.ConnectDatabase(); err != nil {
		log.Fatal(err)
	}
	if err := configuration.AutoMigrate(dbConfig.GetDatabase()); err != nil {
		log.Fatal(err)
	}

	var redisConfig configuration.RedisConfiguration
	redisConfig.Connect()
	defer redisConfig.Disconnect()

	seeder := newLocationSeeder(
		client.NewDefaultGoongClient(),
		repository.NewLocationRepository(dbConfig.GetDatabase()),
		repository.NewPlaceLocationStore(redisConfig.Client()),
	)
	report, err := seeder.seedFromCoords(context.Background(), coords)
	if err != nil {
		if errors.Is(err, client.ErrMissingAPIKey) {
			log.Fatal("set GOONG_MAP_CALC_API_KEY")
		}
		if report != nil {
			printSeedReport(report)
		}
		log.Fatal(err)
	}
	printSeedReport(report)
	if report.Wrote() == 0 {
		log.Print("no rows written — already in db/cache, empty geocode, or unmapped detail")
	}
}

func printSeedReport(report *seedReport) {
	if report == nil {
		return
	}
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

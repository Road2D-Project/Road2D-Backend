package service_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"Road-To-Destination-BE/middleware"
	authModel "Road-To-Destination-BE/module/authentication/model"
	mapRequest "Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/share/configuration"
	"Road-To-Destination-BE/module/share/configuration/version"
	"Road-To-Destination-BE/module/trip/controller"
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/repository"
	tripservice "Road-To-Destination-BE/module/trip/service"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/customValidator"
	"Road-To-Destination-BE/utils/enum"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// The opt-in DSN needs CREATE SCHEMA permission. Each run owns a separate schema
// and removes it on completion, so existing application tables are untouched.
func planningPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("ROAD2D_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set ROAD2D_TEST_POSTGRES_DSN to run PostgreSQL planning tests")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid PostgreSQL test configuration")
	}
	config.ConnectTimeout = 5 * time.Second
	admin := stdlib.OpenDB(*config)
	schema := "planning_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.ExecContext(t.Context(), `CREATE SCHEMA "`+schema+`"`); err != nil {
		admin.Close()
		t.Fatalf("create isolated PostgreSQL schema: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(`DROP SCHEMA "` + schema + `" CASCADE`); err != nil {
			t.Errorf("remove test schema: %v", err)
		}
		admin.Close()
	})
	config.RuntimeParams["search_path"] = schema
	connection := stdlib.OpenDB(*config)
	t.Cleanup(func() { connection.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: connection}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := configuration.AutoMigrate(db); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}
	return db
}

func postgresPlanningService(db *gorm.DB, routes tripservice.LegRouter) *tripservice.PlanningService {
	transaction := func(ctx context.Context, run func(tripservice.PlanningStore) error) error {
		return repository.NewPlanningRepository(db).Transaction(ctx, func(records *repository.PlanningRepository) error {
			return run(records)
		})
	}
	return tripservice.NewPlanningService(transaction, routes, routes, repository.NewLocationRepository(db))
}

func planningUser(t *testing.T, db *gorm.DB) uuid.UUID {
	t.Helper()
	user := authModel.User{Username: uuid.NewString(), Email: uuid.NewString() + "@example.test", Password: "unused-test-hash"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	return user.ID
}

func planningTestTrip(t *testing.T, db *gorm.DB, owner uuid.UUID) uuid.UUID {
	t.Helper()
	trip := &model.Trip{OwnerID: owner, Name: "Planning test", Status: enum.TripPlanning, MemberLimit: 10,
		InviteToken: uuid.NewString(), IdempotencyKey: uuid.NewString(), IdempotencyHash: "test"}
	if err := repository.NewTripRepository(db).CreateTripWithMembers(t.Context(), trip,
		[]model.TripMember{{UserID: owner, Role: enum.TripRoleLeader, Status: enum.MembershipActive}}, nil); err != nil {
		t.Fatal(err)
	}
	return trip.ID
}

func planningPin(t *testing.T, db *gorm.DB, owner uuid.UUID) model.Destination {
	t.Helper()
	pin := model.Destination{CreatedByUserID: &owner, Name: "pin", Lat: 10, Lng: 106, Status: enum.Editing}
	if err := db.Create(&pin).Error; err != nil {
		t.Fatal(err)
	}
	return pin
}

func TestPlanningPostgres(t *testing.T) {
	db := planningPostgres(t)
	svc := postgresPlanningService(db, nil)

	t.Run("fork records creator", func(t *testing.T) {
		owner := planningUser(t, db)
		location := model.Location{Name: "Catalog place", Lat: 10, Lng: 106}
		if err := db.Create(&location).Error; err != nil {
			t.Fatal(err)
		}
		created, err := svc.ForkLocation(t.Context(), owner, location.ID, request.ForkLocationRequest{})
		if err != nil {
			t.Fatal(err)
		}
		pin, err := svc.GetDestination(t.Context(), owner, created.DestinationId)
		if err != nil || pin.CreatedByUserID == nil || *pin.CreatedByUserID != owner {
			t.Fatalf("fork creator: pin=%+v err=%v", pin, err)
		}
	})

	t.Run("HTTP patch invalidates every affected trip cache", func(t *testing.T) {
		owner, outsider := planningUser(t, db), planningUser(t, db)
		actorID := owner
		first, second := planningTestTrip(t, db, owner), planningTestTrip(t, db, owner)
		pin := planningPin(t, db, owner)
		for _, tripID := range []uuid.UUID{first, second} {
			if _, err := svc.AddDraftDestination(t.Context(), owner, tripID, pin.ID); err != nil {
				t.Fatal(err)
			}
		}
		server := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: server.Addr()})
		t.Cleanup(func() { client.Close() })
		cache := middleware.NewResponseCache(client)
		validation := validator.New()
		customValidator.ValidatorRegistrar(validation)
		ctrl := controller.NewTripController(db, client, validation, nil, cache, nil, nil, nil)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("currentUser", &authModel.User{Base: utils.Base{ID: actorID}})
			c.Next()
		})
		router.PATCH("/planning/destination/:destinationId", cache.MarkChanged(), ctrl.HandlePatchDestination())
		router.GET("/planning/destination/:destinationId", ctrl.HandleGetDestination())
		var keys []string
		for _, tripID := range []uuid.UUID{first, second} {
			for _, name := range []string{"graph", "draft", "travels"} {
				key := "resp:" + name + ":" + tripID.String()
				keys = append(keys, key)
				server.Set(key, "old response")
			}
		}
		url := "/planning/destination/" + pin.ID.String()
		requestBody := httptest.NewRequest(http.MethodPatch, url, strings.NewReader(`{"name":"updated via HTTP"}`))
		requestBody.Header.Set("Content-Type", "application/json")
		result := httptest.NewRecorder()
		router.ServeHTTP(result, requestBody)
		if result.Code != http.StatusOK {
			t.Fatalf("patch status=%d body=%s", result.Code, result.Body.String())
		}
		for _, key := range keys {
			if value, err := server.Get(key); err != nil || value != "changed" {
				t.Fatalf("stale %s cache: value=%q err=%v", key, value, err)
			}
		}
		actorID = outsider
		result = httptest.NewRecorder()
		router.ServeHTTP(result, httptest.NewRequest(http.MethodGet, url, nil))
		if result.Code != http.StatusForbidden {
			t.Fatalf("foreign GET status=%d body=%s", result.Code, result.Body.String())
		}
	})

	t.Run("ownership and foreign destination entry points", func(t *testing.T) {
		owner, other := planningUser(t, db), planningUser(t, db)
		tripID := planningTestTrip(t, db, owner)
		pin := planningPin(t, db, other)
		if _, err := svc.GetDestination(t.Context(), owner, pin.ID); !errors.Is(err, repository.ErrDestinationForbidden) {
			t.Fatalf("foreign direct read: %v", err)
		}
		name := "unauthorized"
		if _, _, err := svc.PatchDestination(t.Context(), owner, pin.ID, request.PatchDestinationRequest{Name: &name}); !errors.Is(err, repository.ErrDestinationForbidden) {
			t.Fatalf("foreign patch: %v", err)
		}
		if _, err := svc.DeleteDestination(t.Context(), owner, pin.ID); !errors.Is(err, repository.ErrDestinationForbidden) {
			t.Fatalf("foreign delete: %v", err)
		}
		if _, err := svc.AddDraftDestination(t.Context(), owner, tripID, pin.ID); !errors.Is(err, repository.ErrDestinationForbidden) {
			t.Fatalf("foreign draft insert: %v", err)
		}
		graph := request.SetTripGraphRequest{Branches: [][]uuid.UUID{{pin.ID}}, OpenTail: []bool{false}}
		if _, err := svc.SetTripGraph(t.Context(), owner, tripID, graph); !errors.Is(err, repository.ErrDestinationForbidden) {
			t.Fatalf("foreign graph insert: %v", err)
		}
		if _, err := svc.PreviewBranch(t.Context(), owner, tripID, []request.ComputeBranchPoint{{DestinationID: &pin.ID}, {DestinationID: &pin.ID}}); !errors.Is(err, repository.ErrDestinationForbidden) {
			t.Fatalf("foreign preview: %v", err)
		}
		ownPin := planningPin(t, db, owner)
		if _, err := svc.ComputeTravels(t.Context(), owner, tripID, [][]uuid.UUID{{ownPin.ID, pin.ID}}); !errors.Is(err, repository.ErrDestinationForbidden) {
			t.Fatalf("foreign travel graph: %v", err)
		}
	})

	t.Run("draft lifecycle and creator rights", func(t *testing.T) {
		owner, member := planningUser(t, db), planningUser(t, db)
		tripID := planningTestTrip(t, db, owner)
		if err := db.Create(&model.TripMember{TripID: tripID, UserID: member, Role: enum.TripRoleMember, Status: enum.MembershipActive}).Error; err != nil {
			t.Fatal(err)
		}
		pin := planningPin(t, db, member)
		if _, err := svc.AddDraftDestination(t.Context(), member, tripID, pin.ID); err != nil {
			t.Fatalf("planning trip draft: %v", err)
		}
		name := "my suggestion"
		if _, affected, err := svc.PatchDestination(t.Context(), member, pin.ID, request.PatchDestinationRequest{Name: &name}); err != nil || len(affected) != 1 {
			t.Fatalf("creator edit in draft: affected=%v err=%v", affected, err)
		}
		if err := repository.NewTripRepository(db).CompleteTrip(t.Context(), tripID); err != nil {
			t.Fatal(err)
		}
		if _, _, err := svc.PatchDestination(t.Context(), member, pin.ID, request.PatchDestinationRequest{Name: &name}); !errors.Is(err, repository.ErrDestinationInUse) {
			t.Fatalf("completed draft edit: %v", err)
		}
		if _, err := svc.DeleteDestination(t.Context(), owner, pin.ID); !errors.Is(err, repository.ErrDestinationInUse) {
			t.Fatalf("completed draft delete: %v", err)
		}
		another := planningPin(t, db, member)
		if _, err := svc.AddDraftDestination(t.Context(), member, tripID, another.ID); !errors.Is(err, repository.ErrInvalidTripGraph) {
			t.Fatalf("completed draft insert: %v", err)
		}
	})

	t.Run("historical travel guards and coordinate invalidation", func(t *testing.T) {
		owner := planningUser(t, db)
		tripID := planningTestTrip(t, db, owner)
		from, to := planningPin(t, db, owner), planningPin(t, db, owner)
		travel := model.Travel{TripID: tripID, FromDestinationID: from.ID, ToDestinationID: to.ID, Vehicle: enum.BIKE,
			Polyline: "old", LastComputedAt: time.Now(), IsFrozen: true}
		if err := db.Create(&travel).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := svc.DeleteDestination(t.Context(), owner, from.ID); !errors.Is(err, repository.ErrDestinationInUse) {
			t.Fatalf("travel-only history delete: %v", err)
		}
		if err := db.Model(&travel).Update("is_frozen", false).Error; err != nil {
			t.Fatal(err)
		}
		lat := 11.0
		if _, affected, err := svc.PatchDestination(t.Context(), owner, from.ID, request.PatchDestinationRequest{Lat: &lat}); err != nil || len(affected) != 1 || affected[0] != tripID {
			t.Fatalf("coordinate edit: affected=%v err=%v", affected, err)
		}
		var count int64
		if err := db.Model(&model.Travel{}).Where("id = ?", travel.ID).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("stale travel count=%d err=%v", count, err)
		}
	})

	t.Run("shared destination needs every trip permission", func(t *testing.T) {
		owner, other := planningUser(t, db), planningUser(t, db)
		first, second := planningTestTrip(t, db, owner), planningTestTrip(t, db, other)
		if err := db.Create(&model.TripMember{TripID: second, UserID: owner, Role: enum.TripRoleMember, Status: enum.MembershipActive}).Error; err != nil {
			t.Fatal(err)
		}
		pin := planningPin(t, db, owner)
		if _, err := svc.AddDraftDestination(t.Context(), owner, first, pin.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.AddDraftDestination(t.Context(), owner, second, pin.ID); err != nil {
			t.Fatal(err)
		}
		graph := request.SetTripGraphRequest{Branches: [][]uuid.UUID{{pin.ID}}, OpenTail: []bool{false}}
		if _, err := svc.SetTripGraph(t.Context(), other, second, graph); err != nil {
			t.Fatal(err)
		}
		name := "shared"
		if _, _, err := svc.PatchDestination(t.Context(), owner, pin.ID, request.PatchDestinationRequest{Name: &name}); !errors.Is(err, repository.ErrDestinationForbidden) {
			t.Fatalf("member changed shared route: %v", err)
		}
		if err := db.Model(&model.TripMember{}).Where("trip_id = ? AND user_id = ?", second, owner).Update("role", enum.TripRoleLeader).Error; err != nil {
			t.Fatal(err)
		}
		if _, affected, err := svc.PatchDestination(t.Context(), owner, pin.ID, request.PatchDestinationRequest{Name: &name}); err != nil || len(affected) != 2 {
			t.Fatalf("leader shared edit: affected=%v err=%v", affected, err)
		}
	})

	t.Run("branch writes roll back together", func(t *testing.T) {
		owner := planningUser(t, db)
		tripID := planningTestTrip(t, db, owner)
		a, b, c, d := planningPin(t, db, owner), planningPin(t, db, owner), planningPin(t, db, owner), planningPin(t, db, owner)
		graph := request.SetTripGraphRequest{Branches: [][]uuid.UUID{{a.ID, d.ID, b.ID, c.ID}, {a.ID, d.ID, c.ID}}, OpenTail: []bool{false, false}}
		if _, err := svc.SetTripGraph(t.Context(), owner, tripID, graph); err != nil {
			t.Fatal(err)
		}
		stored, err := repository.NewTripBranchRepository(db).FindTripWithBranches(t.Context(), tripID)
		if err != nil {
			t.Fatal(err)
		}
		var failingBranch uuid.UUID
		for _, branch := range stored.Branches {
			if branch.SplitFromDestinationID != nil {
				failingBranch = branch.ID
			}
		}
		if failingBranch == uuid.Nil {
			t.Fatal("missing subbranch fixture")
		}
		statement := fmt.Sprintf(`CREATE FUNCTION reject_branch_write() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN IF NEW.trip_branch_id = '%s'::uuid THEN RAISE EXCEPTION 'injected branch failure'; END IF; RETURN NEW; END $$`, failingBranch)
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(`CREATE TRIGGER reject_branch_write BEFORE INSERT ON branch_destinations FOR EACH ROW EXECUTE FUNCTION reject_branch_write()`).Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			db.Exec(`DROP TRIGGER reject_branch_write ON branch_destinations`)
			db.Exec(`DROP FUNCTION reject_branch_write()`)
		})
		if _, err := svc.DeleteDestination(t.Context(), owner, d.ID); err == nil {
			t.Fatal("expected injected write failure")
		}
		var count int64
		if err := db.Model(&model.BranchDestination{}).Where("destination_id = ?", d.ID).Count(&count).Error; err != nil || count != 2 {
			t.Fatalf("partial graph committed: references=%d err=%v", count, err)
		}
		if _, err := svc.GetDestination(t.Context(), owner, d.ID); err != nil {
			t.Fatalf("pin deleted after failed rewrite: %v", err)
		}
	})

	t.Run("concurrent draft inserts preserve both pins", func(t *testing.T) {
		owner := planningUser(t, db)
		tripID := planningTestTrip(t, db, owner)
		first, second := planningPin(t, db, owner), planningPin(t, db, owner)
		start := make(chan struct{})
		results := make(chan error, 2)
		var workers sync.WaitGroup
		for _, id := range []uuid.UUID{first.ID, second.ID} {
			workers.Go(func() {
				<-start
				_, err := svc.AddDraftDestination(t.Context(), owner, tripID, id)
				results <- err
			})
		}
		close(start)
		workers.Wait()
		for range 2 {
			if err := <-results; err != nil {
				t.Fatal(err)
			}
		}
		listed, err := tripservice.NewTripBranchService(repository.NewTripBranchRepository(db), repository.NewTripRepository(db)).ListDraftDestinations(t.Context(), tripID)
		if err != nil || len(listed.Destinations) != 2 {
			t.Fatalf("lost concurrent insert: result=%+v err=%v", listed, err)
		}
	})

	t.Run("migration preserves unattributed legacy pins", func(t *testing.T) {
		if err := db.Exec(`ALTER TABLE destinations DROP COLUMN created_by_user_id`).Error; err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if err := version.V11_AddDestinationCreator(db); err != nil {
				t.Fatal(err)
			}
		}
		var pin model.Destination
		if err := db.First(&pin).Error; err != nil || pin.CreatedByUserID != nil {
			t.Fatalf("legacy creator: %v err=%v", pin.CreatedByUserID, err)
		}
	})
}

type blockedPlanningRouter struct {
	started chan struct{}
	release chan struct{}
}

func (r *blockedPlanningRouter) Route(ctx context.Context, _ mapRequest.DirectionRequest) (*model.Leg, error) {
	close(r.started)
	select {
	case <-r.release:
		return &model.Leg{Vehicle: enum.BIKE, Polyline: "computed", LastComputedAt: time.Now()}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestPlanningPostgresRejectsStaleProviderResult(t *testing.T) {
	db := planningPostgres(t)
	owner := planningUser(t, db)
	tripID := planningTestTrip(t, db, owner)
	from, to := planningPin(t, db, owner), planningPin(t, db, owner)
	router := &blockedPlanningRouter{started: make(chan struct{}), release: make(chan struct{})}
	svc := postgresPlanningService(db, router)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := svc.ComputeTravels(ctx, owner, tripID, [][]uuid.UUID{{from.ID, to.ID}})
		result <- err
	}()
	select {
	case <-router.started:
	case <-ctx.Done():
		t.Fatal("provider did not start")
	}
	lat := 11.0
	if _, _, err := svc.PatchDestination(ctx, owner, from.ID, request.PatchDestinationRequest{Lat: &lat}); err != nil {
		t.Fatalf("coordinate update blocked by provider: %v", err)
	}
	close(router.release)
	if err := <-result; !errors.Is(err, repository.ErrPlanningConflict) {
		t.Fatalf("stale result: %v", err)
	}
	var count int64
	if err := db.Model(&model.Travel{}).Where("trip_id = ?", tripID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("stale result persisted: count=%d err=%v", count, err)
	}
}

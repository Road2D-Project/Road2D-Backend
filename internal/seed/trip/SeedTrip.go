package trip

import (
	"context"
	"errors"
	"fmt"

	"Road-To-Destination-BE/internal/seed/user"
	authModel "Road-To-Destination-BE/module/authentication/model"
	authRequest "Road-To-Destination-BE/module/authentication/model/request"
	authRepo "Road-To-Destination-BE/module/authentication/repository"
	authService "Road-To-Destination-BE/module/authentication/service"
	tripRequest "Road-To-Destination-BE/module/trip/model/request"
	tripModel "Road-To-Destination-BE/module/trip/model/response"
	tripRepo "Road-To-Destination-BE/module/trip/repository"
	tripService "Road-To-Destination-BE/module/trip/service"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Options is the seeder input. Empty MemberUsernames seats every bundled fake
// rider except the owner. Empty LocationIDs uses the earliest catalog places.
type Options struct {
	Name            string
	Note            string
	TripType        enum.TripType
	Visibility      bool
	MemberUsernames []string
	LocationIDs     []uuid.UUID
}

// Report is the non-sensitive trip plus the join token. Passwords are omitted.
type Report struct {
	Trip        tripModel.TripResponse         `json:"trip"`
	InviteToken string                         `json:"inviteToken"`
	JoinPath    string                         `json:"joinPath"`
	Members     []tripModel.TripMemberResponse `json:"members"`
	Pins        []ForkedPin                    `json:"pins"`
}

// Seed registers the default user when missing, seats the member list, and
// writes one main branch from location ids (forked in that order).
func Seed(ctx context.Context, db *gorm.DB, redisClient *redis.Client, opts Options) (*Report, error) {
	if opts.Name == "" {
		opts.Name = "Seed trip"
	}
	if opts.Note == "" {
		opts.Note = "Main branch of the chosen locations"
	}
	owner, err := defaultOwner(ctx, db)
	if err != nil {
		return nil, err
	}
	memberIDs, err := memberUserIDs(ctx, db, owner, opts.MemberUsernames)
	if err != nil {
		return nil, err
	}
	tripType := opts.TripType
	trips := tripService.NewTripService(
		tripRepo.NewTripRepository(db),
		authRepo.NewUserRepository(db),
		tripRepo.NewTripMemberRepository(db),
		tripRepo.NewTripMemberRepository(db),
		tripRepo.NewTripMemberStore(redisClient),
		tripRepo.NewLocationRepository(db),
	)
	createdTrip, err := trips.CreateTrip(ctx, owner, tripRequest.CreateTripRequest{
		Name:          opts.Name,
		Note:          opts.Note,
		TripType:      &tripType,
		Visibility:    opts.Visibility,
		MemberUserIDs: memberIDs,
	})
	if err != nil {
		return nil, err
	}
	link, err := trips.CreateInviteLink(ctx, createdTrip.ID, false)
	if err != nil {
		return nil, err
	}
	roster, err := tripService.NewTripMemberService(
		tripRepo.NewTripMemberRepository(db),
		tripRepo.NewTripMemberStore(redisClient),
	).ListActiveMembers(ctx, createdTrip.ID)
	if err != nil {
		return nil, err
	}

	places := tripService.NewLocationService(
		tripRepo.NewDestinationRepository(db),
		tripRepo.NewLocationRepository(db),
	)
	pins, err := ForkLocations(ctx, places, db, opts.LocationIDs)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(pins))
	for i := range pins {
		ids[i] = pins[i].ID
	}
	graph, err := ComposeMainBranch(ids)
	if err != nil {
		return nil, err
	}
	branches := tripService.NewTripBranchService(
		tripRepo.NewTripBranchRepository(db),
		tripRepo.NewTripRepository(db),
	)
	if _, err := branches.SetTripGraph(ctx, createdTrip.ID, graph, enum.TripRoleLeader); err != nil {
		return nil, fmt.Errorf("set trip graph: %w", err)
	}
	return &Report{
		Trip:        *createdTrip,
		InviteToken: link.Token,
		JoinPath:    link.JoinPath,
		Members:     roster.Members,
		Pins:        pins,
	}, nil
}

func defaultOwner(ctx context.Context, db *gorm.DB) (*authModel.User, error) {
	creds, err := user.DefaultUser()
	if err != nil {
		return nil, err
	}
	users := authRepo.NewUserRepository(db)
	found, err := users.FindUserByUsername(ctx, creds.Username)
	if err == nil {
		return found, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if _, err := authService.NewRegisterService(users).Register(authRequest.RegisterRequest{
		Username:        creds.Username,
		Email:           creds.Email,
		Password:        creds.Password,
		ConfirmPassword: creds.Password,
	}, ctx); err != nil {
		return nil, err
	}
	return users.FindUserByUsername(ctx, creds.Username)
}

func memberUserIDs(ctx context.Context, db *gorm.DB, owner *authModel.User, names []string) ([]uuid.UUID, error) {
	fakes, err := user.DefaultFakeUsers()
	if err != nil {
		return nil, err
	}
	users := authRepo.NewUserRepository(db)
	if _, err := user.SeedFakeUsers(ctx, authService.NewRegisterService(users), users, fakes); err != nil {
		return nil, err
	}
	if len(names) == 0 {
		for _, account := range fakes {
			if account.Username == owner.Username {
				continue
			}
			names = append(names, account.Username)
		}
	}
	ids := make([]uuid.UUID, 0, len(names))
	seen := map[uuid.UUID]struct{}{}
	for _, name := range names {
		if name == "" || name == owner.Username {
			continue
		}
		found, err := users.FindUserByUsername(ctx, name)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("member %q was not found", name)
			}
			return nil, err
		}
		if found.ID == owner.ID {
			continue
		}
		if _, ok := seen[found.ID]; ok {
			continue
		}
		seen[found.ID] = struct{}{}
		ids = append(ids, found.ID)
	}
	if len(ids) == 0 {
		return nil, errors.New("name at least one member besides the trip leader")
	}
	return ids, nil
}

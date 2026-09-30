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
	groupRequest "Road-To-Destination-BE/module/group/model/request"
	groupRepo "Road-To-Destination-BE/module/group/repository"
	groupService "Road-To-Destination-BE/module/group/service"
	tripRequest "Road-To-Destination-BE/module/trip/model/request"
	tripRepo "Road-To-Destination-BE/module/trip/repository"
	tripService "Road-To-Destination-BE/module/trip/service"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	seedGroupName = "Seed group"
	seedTripName  = "Seed trip"
)

// Report is what one seeder trip run created.
type Report struct {
	Username string
	GroupID  uuid.UUID
	TripID   uuid.UUID
	Pins     []ForkedPin
}

// Seed registers the default user when missing, then creates a group and a trip
// owned by that user. It forks up to DestinationLimit locations and writes them
// as the trip's main branch.
func Seed(ctx context.Context, db *gorm.DB, redisClient *redis.Client) (*Report, error) {
	owner, err := defaultOwner(ctx, db)
	if err != nil {
		return nil, err
	}

	groups := groupService.NewGroupService(
		groupRepo.NewGroupRepository(db),
		authRepo.NewUserRepository(db),
		groupRepo.NewGroupMemberRepository(db),
		groupRepo.NewGroupMemberStore(redisClient),
	)
	createdGroup, err := groups.CreateGroup(ctx, owner, groupRequest.CreateGroupRequest{
		Name:        seedGroupName,
		Description: "Created by seeder trip",
	})
	if err != nil {
		return nil, err
	}

	trips := tripService.NewTripService(
		tripRepo.NewTripRepository(db),
		groupRepo.NewGroupRepository(db),
		groupRepo.NewGroupMemberRepository(db),
		tripRepo.NewTripMemberRepository(db),
		tripRepo.NewTripMemberStore(redisClient),
	)
	createdTrip, err := trips.CreateTrip(ctx, owner, tripRequest.CreateTripRequest{
		GroupID: createdGroup.ID,
		Name:    seedTripName,
		Note:    "Main branch of the first forked destinations",
	})
	if err != nil {
		return nil, err
	}

	places := tripService.NewLocationService(
		tripRepo.NewDestinationRepository(db),
		tripRepo.NewLocationRepository(db),
	)
	pins, err := ForkDestinations(ctx, places, db)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(pins))
	for i := range pins {
		names[i] = pins[i].Name
	}
	graph, err := ComposeSetTripGraph(db, names)
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
		Username: owner.Username,
		GroupID:  createdGroup.ID,
		TripID:   createdTrip.ID,
		Pins:     pins,
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

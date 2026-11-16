package tracking

import (
	"encoding/json"
	"testing"
	"time"

	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/module/tracking/engine"
	"Road-To-Destination-BE/module/tracking/model"
	"Road-To-Destination-BE/module/tracking/model/message"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

type recordSession struct {
	broadcasts []recorded
}

type recorded struct {
	msgType string
	payload any
}

func (s *recordSession) Send(uuid.UUID, string, any) {}

func (s *recordSession) Broadcast(msgType string, payload any) {
	s.broadcasts = append(s.broadcasts, recorded{msgType: msgType, payload: payload})
}

func (s *recordSession) BroadcastLatest(string, string, any) {}

func (s *recordSession) IsOnline(uuid.UUID) bool { return true }

func TestPositionCarriesBatteryAndSignal(t *testing.T) {
	leader := uuid.New()
	coord, session := stayFixture(leader, map[uuid.UUID]enum.DriveStatus{leader: enum.OnROUTE})
	body := json.RawMessage(`{"Lat":11.1,"Lng":106.2,"BatteryPercent":80,"SignalDbm":-90}`)
	coord.OnMessage(session, leader, string(message.Position), body)

	prg := coord.progresses[leader]
	if prg.Lat != 11.1 || prg.Lng != 106.2 || prg.BatteryPercent != 80 || prg.SignalBars != 3 {
		t.Fatalf("stored %+v", prg)
	}

	coord.OnTick(session, time.Now())
	if len(session.broadcasts) != 1 || session.broadcasts[0].msgType != "tracking.positions" {
		t.Fatalf("broadcasts %#v", session.broadcasts)
	}
	raw, err := json.Marshal(session.broadcasts[0].payload)
	if err != nil {
		t.Fatal(err)
	}
	var posted []struct {
		BatteryPercent int
		SignalBars     int
	}
	if err := json.Unmarshal(raw, &posted); err != nil {
		t.Fatal(err)
	}
	if len(posted) != 1 || posted[0].BatteryPercent != 80 || posted[0].SignalBars != 3 {
		t.Fatalf("posted %+v", posted)
	}
}

func TestLeaderStayHidesRidersUntilTheyGoPublic(t *testing.T) {
	leader := uuid.New()
	member := uuid.New()
	done := uuid.New()
	coord, session := stayFixture(leader, map[uuid.UUID]enum.DriveStatus{
		leader: enum.OnROUTE,
		member: enum.DEVIATED,
		done:   enum.COMPLETED,
	})

	coord.OnMessage(session, member, string(message.StayMode), json.RawMessage(`true`))
	if coord.tripInformation.Status != enum.TripActive || len(session.broadcasts) != 0 {
		t.Fatalf("member stay status %s broadcasts %d", coord.tripInformation.Status, len(session.broadcasts))
	}

	coord.OnMessage(session, leader, string(message.StayMode), json.RawMessage(`true`))
	if coord.tripInformation.Status != enum.TripStay || len(session.broadcasts) != 1 {
		t.Fatalf("status %s broadcasts %d", coord.tripInformation.Status, len(session.broadcasts))
	}
	if coord.progresses[leader].DriveStatus != enum.StayPrivate || coord.progresses[member].DriveStatus != enum.StayPrivate {
		t.Fatal("resting riders stay visible")
	}
	if coord.progresses[done].DriveStatus != enum.COMPLETED {
		t.Fatal("finished rider was pulled into stay")
	}

	coord.OnTick(session, time.Now())
	if len(session.broadcasts) != 1 {
		t.Fatalf("stay tick broadcasts %d", len(session.broadcasts))
	}

	coord.OnMessage(session, member, string(message.UserStayPublic), json.RawMessage(`true`))
	coord.OnTick(session, time.Now())
	if len(session.broadcasts) != 2 || session.broadcasts[1].msgType != "tracking.positions" {
		t.Fatalf("public tick %#v", session.broadcasts)
	}
	posted := positionUserIDs(t, session.broadcasts[1].payload)
	if len(posted) != 1 || posted[0] != member {
		t.Fatalf("posted %v", posted)
	}

	coord.OnMessage(session, done, string(message.UserStayPublic), json.RawMessage(`true`))
	if coord.progresses[done].DriveStatus != enum.COMPLETED {
		t.Fatal("finished rider became public")
	}
}

func TestLeaveStayRestoresOnlyRestingRiders(t *testing.T) {
	leader := uuid.New()
	member := uuid.New()
	done := uuid.New()
	coord, session := stayFixture(leader, map[uuid.UUID]enum.DriveStatus{
		leader: enum.OnROUTE,
		member: enum.OnROUTE,
		done:   enum.COMPLETED,
	})
	coord.OnMessage(session, leader, string(message.StayMode), json.RawMessage(`true`))
	coord.OnMessage(session, member, string(message.UserStayPublic), json.RawMessage(`true`))

	coord.OnMessage(session, leader, string(message.StayMode), json.RawMessage(`false`))
	if coord.tripInformation.Status != enum.TripActive {
		t.Fatalf("status %s", coord.tripInformation.Status)
	}
	if coord.progresses[leader].DriveStatus != enum.OnROUTE || coord.progresses[member].DriveStatus != enum.OnROUTE {
		t.Fatal("resting riders did not resume")
	}
	if coord.progresses[done].DriveStatus != enum.COMPLETED {
		t.Fatal("finished rider resumed")
	}

	before := len(session.broadcasts)
	coord.OnMessage(session, leader, string(message.StayMode), json.RawMessage(`false`))
	if len(session.broadcasts) != before {
		t.Fatal("repeated leave broadcast")
	}
}

func TestTickLocksTheTripWhenEveryJoinedRiderFinishes(t *testing.T) {
	leader := uuid.New()
	member := uuid.New()
	coord, session := stayFixture(leader, map[uuid.UUID]enum.DriveStatus{
		leader: enum.COMPLETED,
		member: enum.OnROUTE,
	})
	coord.OnTick(session, time.Now())
	if coord.tripInformation.Status != enum.TripActive {
		t.Fatalf("status %s", coord.tripInformation.Status)
	}

	coord.progresses[member].DriveStatus = enum.COMPLETED
	coord.OnTick(session, time.Now())
	if coord.tripInformation.Status != enum.TripCompleted {
		t.Fatalf("status %s", coord.tripInformation.Status)
	}
}

func TestCompletedTripIgnoresStay(t *testing.T) {
	leader := uuid.New()
	coord, session := stayFixture(leader, map[uuid.UUID]enum.DriveStatus{leader: enum.OnROUTE})
	coord.tripInformation.Status = enum.TripCompleted

	coord.OnMessage(session, leader, string(message.StayMode), json.RawMessage(`true`))
	if coord.tripInformation.Status != enum.TripCompleted || len(session.broadcasts) != 0 {
		t.Fatalf("status %s broadcasts %d", coord.tripInformation.Status, len(session.broadcasts))
	}
	if coord.progresses[leader].DriveStatus != enum.OnROUTE {
		t.Fatal("completed trip changed a rider")
	}
}

func TestStayVisibilityIgnoredOutsideStay(t *testing.T) {
	leader := uuid.New()
	coord, session := stayFixture(leader, map[uuid.UUID]enum.DriveStatus{leader: enum.OnROUTE})

	coord.OnMessage(session, leader, string(message.UserStayPublic), json.RawMessage(`true`))
	if coord.progresses[leader].DriveStatus != enum.OnROUTE {
		t.Fatal("visibility changed while the trip was moving")
	}

	missing := uuid.New()
	coord.tripInformation.Status = enum.TripStay
	coord.OnMessage(session, missing, string(message.UserStayPublic), json.RawMessage(`true`))
}

func stayFixture(leader uuid.UUID, drives map[uuid.UUID]enum.DriveStatus) (*TrackingCoordinator, *recordSession) {
	riders := map[uuid.UUID]*model.Rider{
		leader: {UserID: leader, Role: enum.TripRoleLeader},
	}
	progresses := map[uuid.UUID]*engine.TravelProgress{}
	for id, drive := range drives {
		if riders[id] == nil {
			riders[id] = &model.Rider{UserID: id, Role: enum.TripRoleMember}
		}
		progress := engine.NewTravelProgress(id, uuid.Nil)
		progress.DriveStatus = drive
		progress.Lat = 10
		progress.Lng = 106
		// Older than SampleMinInterval so the first test sample is accepted.
		progress.ObservedAt = time.Now().Add(-time.Second)
		progresses[id] = progress
	}
	return &TrackingCoordinator{
		tripInformation: &model.TripInformation{
			Status: enum.TripActive,
			Riders: riders,
		},
		progresses: progresses,
	}, &recordSession{}
}

func positionUserIDs(t *testing.T, payload any) []uuid.UUID {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var posted []struct {
		UserId uuid.UUID
	}
	if err := json.Unmarshal(raw, &posted); err != nil {
		t.Fatal(err)
	}
	ids := make([]uuid.UUID, len(posted))
	for i := range posted {
		ids[i] = posted[i].UserId
	}
	return ids
}

var _ realtime.Session = (*recordSession)(nil)

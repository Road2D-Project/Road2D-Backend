package tracking

import (
	"encoding/json"
	"testing"
	"time"

	"Road-To-Destination-BE/module/tracking/engine"
	"Road-To-Destination-BE/module/tracking/model"
	"Road-To-Destination-BE/module/tracking/model/message"
	"Road-To-Destination-BE/utils/enum"
	"Road-To-Destination-BE/utils/polyline"

	"github.com/google/uuid"
)

type capturedSnapshots struct {
	recordSession
	snapshot model.TripSnapshot
}

func (s *capturedSnapshots) Send(_ uuid.UUID, kind string, payload any) {
	if kind == "tracking.snapshot" {
		s.snapshot = payload.(model.TripSnapshot)
	}
}

type stubRedirectSubmitter struct {
	accept bool
	jobs   []model.RedirectJob
}

func (s *stubRedirectSubmitter) TrySubmit(job model.RedirectJob) bool {
	if !s.accept {
		return false
	}
	s.jobs = append(s.jobs, job)
	return true
}

func TestCheckpointIsIndependentAndRecoveryWaitsForFreshGPS(t *testing.T) {
	coord, _, p := deviationFixture(nil)
	coord.tripInformation.TripId = uuid.New()
	coord.tripInformation.Riders[p.UserID].AssignedBranchID = p.BranchID
	p.Lives = 1
	p.HopIndex = 1
	p.OffRouteSamples = 2
	p.ObservedAt = time.Now().Add(-time.Minute)
	destination := uuid.New()
	p.Arrived = map[uuid.UUID]struct{}{destination: {}}
	checkpoint := coord.captureCheckpoint(time.Now())
	delete(p.Arrived, destination)
	p.Lives = 3
	if len(checkpoint.Riders[p.UserID].Arrived) != 1 || checkpoint.Riders[p.UserID].Lives != 1 {
		t.Fatal("room mutation changed captured checkpoint")
	}
	coord.progresses = make(map[uuid.UUID]*engine.TravelProgress)
	coord.restoreCheckpoint(checkpoint)
	restored := coord.progresses[p.UserID]
	defer restored.StopSnap()
	if restored.HopIndex != 1 || restored.Lives != 1 || restored.OffRouteSamples != 0 || !restored.AwaitingFreshPosition {
		t.Fatalf("restored %+v", restored)
	}
	if restored.Route == nil || restored.Route.Cumulation[0] <= 0 {
		t.Fatal("recovered cursor lost distance traveled on earlier hops")
	}
	session := &capturedSnapshots{}
	coord.OnJoin(session, p.UserID)
	coord.OnTick(session, time.Now())
	if len(session.broadcasts) != 1 || session.broadcasts[0].msgType != "tracking.connection" {
		t.Fatal("historical GPS was processed as a fresh observation")
	}
	if !session.snapshot.Riders[p.UserID.String()].AwaitingFreshPosition {
		t.Fatal("reconnect snapshot did not identify historical position")
	}
}

func TestRecoveredPendingRedirectWaitsForFreshGPSAndUsesNewJob(t *testing.T) {
	coord, session, p := deviationFixture(nil)
	coord.tripInformation.TripId = uuid.New()
	coord.tripInformation.Riders[p.UserID].AssignedBranchID = p.BranchID
	submitter := &stubRedirectSubmitter{accept: true}
	coord.recovery.Redirects = submitter
	coord.enterDeviated(session, p, time.Now().Add(-time.Minute))
	previousJob := submitter.jobs[0]
	checkpoint := coord.captureCheckpoint(time.Now())
	submitter.jobs = nil
	coord.progresses = make(map[uuid.UUID]*engine.TravelProgress)
	coord.restoreCheckpoint(checkpoint)
	restored := coord.progresses[p.UserID]
	coord.advanceRedirectJobs(session, time.Now())
	if len(submitter.jobs) != 0 {
		t.Fatal("recovery requested redirect from historical GPS")
	}
	coord.OnMessage(session, p.UserID, string(message.Position), json.RawMessage(`{"Lat":10.776,"Lng":106.702}`))
	coord.advanceRedirectJobs(session, time.Now())
	if len(submitter.jobs) != 1 || submitter.jobs[0].JobID == previousJob.JobID ||
		submitter.jobs[0].Generation <= previousJob.Generation || submitter.jobs[0].Origin.Lat != 10.776 ||
		!restored.Redirect.Pending {
		t.Fatalf("recovered jobs %+v", submitter.jobs)
	}
}

func TestSnapshotHidesPrivateRiderPositionAndRedirect(t *testing.T) {
	coord, _, p := deviationFixture(nil)
	p.DriveStatus = enum.StayPrivate
	p.ObservedAt = time.Now()
	p.Redirect = &engine.RedirectState{Polyline: "private-route"}
	session := &capturedSnapshots{}
	coord.sendSnapshot(session, uuid.New())
	public := session.snapshot.Riders[p.UserID.String()]
	if public.Lat != nil || public.Lng != nil || public.Redirect != nil {
		t.Fatal("snapshot exposed a resting rider's private position")
	}
	coord.sendSnapshot(session, p.UserID)
	self := session.snapshot.Riders[p.UserID.String()]
	if self.Lat == nil || self.Redirect == nil {
		t.Fatal("owner snapshot omitted their position")
	}
}

func TestRecoveryKeepsRestingRiderPrivateWhenBranchAssignmentChanged(t *testing.T) {
	coord, _, p := deviationFixture(nil)
	coord.tripInformation.TripId = uuid.New()
	coord.tripInformation.Status = enum.TripStay
	p.DriveStatus = enum.StayPrivate
	p.ObservedAt = time.Now()
	checkpoint := coord.captureCheckpoint(time.Now())
	newBranch := uuid.New()
	coord.tripInformation.Riders[p.UserID].AssignedBranchID = newBranch
	coord.tripInformation.Branches[newBranch] = &model.TravelBranch{ID: newBranch}
	coord.restoreCheckpoint(checkpoint)
	session := &capturedSnapshots{}
	coord.sendSnapshot(session, uuid.New())
	if session.snapshot.Riders[p.UserID.String()].Lat != nil {
		t.Fatal("branch reassignment exposed private checkpoint position")
	}
}

func TestRemoteRedirectRejectsOldJobAndAppliesDuplicateOnce(t *testing.T) {
	coord, session, p := deviationFixture(nil)
	coord.tripInformation.TripId = uuid.New()
	submitter := &stubRedirectSubmitter{accept: true}
	coord.recovery.Redirects = submitter
	coord.enterDeviated(session, p, time.Now())
	job := submitter.jobs[0]
	results := make(chan model.RedirectResult, 3)
	coord.redirectResults = results
	result := model.RedirectResult{
		JobID: uuid.New(), TripID: job.TripID, MemberID: job.MemberID, Generation: job.Generation,
		Polyline: polyline.Encode([]polyline.Point{{Lat: beside.Lat, Lng: beside.Lng}, {Lat: stopB.Lat, Lng: stopB.Lng}}),
	}
	results <- result
	coord.drainRemoteRedirectResults()
	coord.applyRedirectResults(session, time.Now())
	if p.Redirect.Route != nil {
		t.Fatal("result from a previous room's job was applied")
	}
	result.JobID = job.JobID
	results <- result
	results <- result
	coord.drainRemoteRedirectResults()
	coord.applyRedirectResults(session, time.Now())
	ready := 0
	for _, posted := range session.broadcasts {
		if posted.msgType == message.OutRedirect && posted.payload.(message.Redirect).Status == message.RedirectReady {
			ready++
		}
	}
	if ready != 1 || p.Redirect.Route == nil {
		t.Fatalf("ready broadcasts = %d", ready)
	}
}

func TestRedirectQueueBackpressureRetriesSameJobThenTimesOut(t *testing.T) {
	coord, session, p := deviationFixture(nil)
	submitter := &stubRedirectSubmitter{}
	coord.recovery.Redirects = submitter
	now := time.Now()
	coord.enterDeviated(session, p, now)
	id := p.Redirect.JobID
	if p.Redirect.Submitted || !p.Redirect.Pending {
		t.Fatal("full queue did not preserve pending request")
	}
	submitter.accept = true
	coord.advanceRedirectJobs(session, now.Add(time.Second))
	if len(submitter.jobs) != 1 || submitter.jobs[0].JobID != id {
		t.Fatal("retry changed logical job identity")
	}
	coord.advanceRedirectJobs(session, now.Add(redirectJobMaxAge))
	if p.Redirect.Pending || !p.Redirect.Failed {
		t.Fatal("lost job stayed pending forever")
	}
}

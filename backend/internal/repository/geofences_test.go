package repository

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"geoentity/backend/internal/entity"
	"geoentity/backend/internal/geofence"
)

func openGeofenceTestStore(t *testing.T) (*SQLite, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "geofences.db")
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, path
}

func circleDefinition(id, name string, enabled bool, radius float64) geofence.Definition {
	center := geofence.Position{106.8, -6.2}
	return geofence.Definition{
		ID: id, Name: name, Enabled: enabled,
		Geometry:  geofence.Geometry{Type: "Circle", Center: &center, RadiusM: &radius},
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func membershipEntity(id, name string, latitude, longitude float64, at time.Time) entity.Entity {
	return entity.Entity{ID: id, Name: name, Type: "facility", Status: "active", Latitude: latitude, Longitude: longitude, CreatedAt: at.UTC(), UpdatedAt: at.UTC()}
}

func updateMembershipEntity(t *testing.T, store *SQLite, id, name string, latitude, longitude float64, at time.Time) {
	t.Helper()
	item, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	item.Name, item.Latitude, item.Longitude, item.UpdatedAt = name, latitude, longitude, at.UTC()
	if err := store.Update(context.Background(), item); err != nil {
		t.Fatal(err)
	}
}

func TestGeofenceMembershipLifecycleAndAppendOnlyEvents(t *testing.T) {
	ctx := context.Background()
	store, _ := openGeofenceTestStore(t)
	fence := circleDefinition("00000000-0000-4000-8000-000000000101", "Depot", true, 250)
	if err := store.CreateGeofence(ctx, fence); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	inside := membershipEntity("00000000-0000-4000-8000-000000000201", "Inside", -6.2, 106.8, base)
	outside := membershipEntity("00000000-0000-4000-8000-000000000202", "Outside", -6.2, 106.81, base)
	for _, item := range []entity.Entity{inside, outside} {
		if err := store.Create(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	secondFence := circleDefinition("00000000-0000-4000-8000-000000000102", "Small Depot", true, 25)
	if err := store.CreateGeofence(ctx, secondFence); err != nil {
		t.Fatal(err)
	}
	items, err := store.EntityGeofences(ctx, inside.ID)
	if err != nil || len(items) != 2 || items[0].State != "inside" || items[1].State != "inside" {
		t.Fatalf("initial inside membership=%+v err=%v", items, err)
	}
	items, err = store.EntityGeofences(ctx, outside.ID)
	if err != nil || len(items) != 2 || items[0].State != "outside" || items[1].State != "outside" {
		t.Fatalf("initial outside membership=%+v err=%v", items, err)
	}
	page, err := store.GeofenceEvents(ctx, fence.ID, GeofenceEventFilters{}, 10, "")
	if err != nil || len(page.Data) != 0 {
		t.Fatalf("initial baselines emitted events: %+v err=%v", page, err)
	}

	updateMembershipEntity(t, store, outside.ID, "Outside", -6.2, 106.8, base.Add(time.Minute))
	updateMembershipEntity(t, store, outside.ID, "Renamed", -6.2, 106.8, base.Add(2*time.Minute)) // unrelated-field update
	updateMembershipEntity(t, store, outside.ID, "Renamed", -6.2, 106.80001, base.Add(3*time.Minute))
	updateMembershipEntity(t, store, outside.ID, "Renamed", -6.2, 106.81, base.Add(4*time.Minute))
	updateMembershipEntity(t, store, outside.ID, "Renamed", -6.2, 106.812, base.Add(5*time.Minute))
	updateMembershipEntity(t, store, outside.ID, "Renamed", -6.2, 106.8, base.Add(6*time.Minute))
	page, err = store.GeofenceEvents(ctx, fence.ID, GeofenceEventFilters{}, 10, "")
	if err != nil || len(page.Data) != 3 {
		t.Fatalf("events=%+v err=%v", page, err)
	}
	if page.Data[0].EventType != "enter" || page.Data[1].EventType != "exit" || page.Data[2].EventType != "enter" {
		t.Fatalf("transition sequence=%+v", page.Data)
	}
	for _, event := range page.Data {
		if event.Source != "coordinate_update" || event.OccurredAt.Location() != time.UTC || event.GeofenceName != "Depot" {
			t.Fatalf("event snapshot/source/time invalid: %+v", event)
		}
	}

	fence.Enabled = false
	fence.UpdatedAt = base.Add(7 * time.Minute)
	if err := store.UpdateGeofence(ctx, fence); err != nil {
		t.Fatal(err)
	}
	items, err = store.EntityGeofences(ctx, outside.ID)
	if err != nil || len(items) != 2 || items[0].State != "inactive" || items[1].State != "inside" {
		t.Fatalf("disabled membership=%+v err=%v", items, err)
	}
	updateMembershipEntity(t, store, outside.ID, "Renamed", -6.2, 106.81, base.Add(8*time.Minute))
	fence.Enabled = true
	fence.Geometry = *parseRepoCircle(t, 5)
	fence.UpdatedAt = base.Add(9 * time.Minute)
	if err := store.UpdateGeofence(ctx, fence); err != nil {
		t.Fatal(err)
	}
	items, err = store.EntityGeofences(ctx, outside.ID)
	if err != nil || len(items) != 2 || items[0].State != "outside" || items[1].State != "outside" {
		t.Fatalf("reenabled/edited baseline=%+v err=%v", items, err)
	}
	page, err = store.GeofenceEvents(ctx, fence.ID, GeofenceEventFilters{}, 10, "")
	if err != nil || len(page.Data) != 3 {
		t.Fatalf("lifecycle edits fabricated events: %+v err=%v", page, err)
	}

	if err := store.Delete(ctx, outside.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EntityGeofences(ctx, outside.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted entity membership read error=%v", err)
	}
	var currentMemberships int
	if err := store.db.QueryRow(`SELECT count(*) FROM entity_geofence_memberships WHERE entity_id=?`, outside.ID).Scan(&currentMemberships); err != nil || currentMemberships != 0 {
		t.Fatalf("entity deletion retained current membership count=%d err=%v", currentMemberships, err)
	}
	page, err = store.GeofenceEvents(ctx, fence.ID, GeofenceEventFilters{}, 10, "")
	if err != nil || len(page.Data) != 3 || page.Data[0].EntityName != "Renamed" {
		t.Fatalf("entity deletion changed event history: %+v err=%v", page, err)
	}
	if err := store.DeleteGeofence(ctx, fence.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow(`SELECT count(*) FROM entity_geofence_memberships WHERE geofence_id=?`, fence.ID).Scan(&currentMemberships); err != nil || currentMemberships != 0 {
		t.Fatalf("geofence deletion retained current membership count=%d err=%v", currentMemberships, err)
	}
	page, err = store.GeofenceEvents(ctx, fence.ID, GeofenceEventFilters{}, 10, "")
	if err != nil || len(page.Data) != 3 || page.Data[0].GeofenceName != "Depot" {
		t.Fatalf("geofence deletion changed event history: %+v err=%v", page, err)
	}
}

func TestImportedEntityGetsSilentInitialMembership(t *testing.T) {
	ctx := context.Background()
	store, _ := openGeofenceTestStore(t)
	fence := circleDefinition("00000000-0000-4000-8000-000000000141", "Import zone", true, 250)
	if err := store.CreateGeofence(ctx, fence); err != nil {
		t.Fatal(err)
	}
	item := membershipEntity("00000000-0000-4000-8000-000000000241", "Imported", -6.2, 106.8, time.Now().UTC())
	if _, _, err := store.CreateImportedEntities(ctx, "csv", "batch-1", []ImportRecord{{Entity: item}}); err != nil {
		t.Fatal(err)
	}
	memberships, err := store.EntityGeofences(ctx, item.ID)
	if err != nil || len(memberships) != 1 || memberships[0].State != "inside" {
		t.Fatalf("import membership=%+v err=%v", memberships, err)
	}
	page, err := store.GeofenceEvents(ctx, fence.ID, GeofenceEventFilters{}, 10, "")
	if err != nil || len(page.Data) != 0 {
		t.Fatalf("import baseline emitted transitions: %+v err=%v", page, err)
	}
}

func parseRepoCircle(t *testing.T, radius float64) *geofence.Geometry {
	t.Helper()
	item := circleDefinition("00000000-0000-4000-8000-000000000999", "temporary", true, radius)
	return &item.Geometry
}

func TestGeofenceEventPaginationFiltersAndDeletedFenceHistory(t *testing.T) {
	ctx := context.Background()
	store, _ := openGeofenceTestStore(t)
	fence := circleDefinition("00000000-0000-4000-8000-000000000111", "Paged", true, 250)
	if err := store.CreateGeofence(ctx, fence); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	item := membershipEntity("00000000-0000-4000-8000-000000000211", "Mover", -6.2, 106.81, base)
	if err := store.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		longitude := 106.8
		if i%2 == 1 {
			longitude = 106.81
		}
		updateMembershipEntity(t, store, item.ID, item.Name, -6.2, longitude, base.Add(time.Duration(i+1)*time.Minute))
	}
	first, err := store.GeofenceEvents(ctx, fence.ID, GeofenceEventFilters{EntityID: item.ID}, 2, "")
	if err != nil || len(first.Data) != 2 || first.Meta.NextCursor == "" {
		t.Fatalf("first page=%+v err=%v", first, err)
	}
	second, err := store.GeofenceEvents(ctx, fence.ID, GeofenceEventFilters{EntityID: item.ID}, 2, first.Meta.NextCursor)
	if err != nil || len(second.Data) != 1 || second.Meta.NextCursor != "" {
		t.Fatalf("second page=%+v err=%v", second, err)
	}
	if _, err := store.GeofenceEvents(ctx, fence.ID, GeofenceEventFilters{}, 2, first.Meta.NextCursor); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("filter-mismatched cursor error=%v", err)
	}
	from := first.Data[1].OccurredAt.Add(time.Nanosecond)
	filtered, err := store.GeofenceEvents(ctx, fence.ID, GeofenceEventFilters{From: &from}, 10, "")
	if err != nil || len(filtered.Data) != 1 {
		t.Fatalf("time-filtered events=%+v err=%v", filtered, err)
	}
	if err := store.DeleteGeofence(ctx, fence.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GeofenceEvents(ctx, "00000000-0000-4000-8000-000000000112", GeofenceEventFilters{}, 10, ""); !errors.Is(err, ErrGeofenceNotFound) {
		t.Fatalf("unknown geofence history error=%v", err)
	}
}

func TestCoordinateTransitionFailureRollsBackEntityMembershipAndEvent(t *testing.T) {
	for _, test := range []struct {
		name    string
		trigger string
	}{
		{"event insert", `CREATE TRIGGER fail_geofence_event BEFORE INSERT ON geofence_events BEGIN SELECT RAISE(ABORT,'injected'); END`},
		{"membership update", `CREATE TRIGGER fail_membership_update BEFORE UPDATE ON entity_geofence_memberships BEGIN SELECT RAISE(ABORT,'injected'); END`},
		{"activity insert", `CREATE TRIGGER fail_activity_insert BEFORE INSERT ON activity_events BEGIN SELECT RAISE(ABORT,'injected'); END`},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			store, _ := openGeofenceTestStore(t)
			fence := circleDefinition("00000000-0000-4000-8000-000000000121", "Rollback", true, 250)
			if err := store.CreateGeofence(ctx, fence); err != nil {
				t.Fatal(err)
			}
			base := time.Now().UTC()
			item := membershipEntity("00000000-0000-4000-8000-000000000221", "Mover", -6.2, 106.81, base)
			if err := store.Create(ctx, item); err != nil {
				t.Fatal(err)
			}
			if _, err := store.db.Exec(test.trigger); err != nil {
				t.Fatal(err)
			}
			item.Longitude = 106.8
			item.UpdatedAt = base.Add(time.Minute)
			if err := store.Update(ctx, item); err == nil {
				t.Fatal("expected injected transaction failure")
			}
			got, err := store.Get(ctx, item.ID)
			if err != nil || got.Longitude != 106.81 {
				t.Fatalf("entity coordinate was not rolled back: %+v err=%v", got, err)
			}
			memberships, err := store.EntityGeofences(ctx, item.ID)
			if err != nil || len(memberships) != 1 || memberships[0].State != "outside" {
				t.Fatalf("membership was not rolled back: %+v err=%v", memberships, err)
			}
			page, err := store.GeofenceEvents(ctx, fence.ID, GeofenceEventFilters{}, 10, "")
			if err != nil || len(page.Data) != 0 {
				t.Fatalf("event was not rolled back: %+v err=%v", page, err)
			}
		})
	}
}

func TestConcurrentIdenticalCoordinateUpdatesEmitSingleTransition(t *testing.T) {
	ctx := context.Background()
	store, _ := openGeofenceTestStore(t)
	fence := circleDefinition("00000000-0000-4000-8000-000000000131", "Concurrent", true, 250)
	if err := store.CreateGeofence(ctx, fence); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC()
	item := membershipEntity("00000000-0000-4000-8000-000000000231", "Mover", -6.2, 106.81, base)
	if err := store.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	const writers = 12
	errs := make(chan error, writers)
	var wait sync.WaitGroup
	for i := 0; i < writers; i++ {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			update := item
			update.Longitude = 106.8
			update.UpdatedAt = base.Add(time.Duration(i+1) * time.Second)
			errs <- store.Update(ctx, update)
		}(i)
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	page, err := store.GeofenceEvents(ctx, fence.ID, GeofenceEventFilters{}, 10, "")
	if err != nil || len(page.Data) != 1 || page.Data[0].EventType != "enter" {
		t.Fatalf("concurrent repeated updates produced events=%+v err=%v", page, err)
	}
}

func TestConcurrentConflictingCoordinateUpdatesKeepTransitionSequenceConsistent(t *testing.T) {
	ctx := context.Background()
	store, _ := openGeofenceTestStore(t)
	fence := circleDefinition("00000000-0000-4000-8000-000000000151", "Conflict", true, 250)
	if err := store.CreateGeofence(ctx, fence); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC()
	item := membershipEntity("00000000-0000-4000-8000-000000000251", "Concurrent", -6.2, 106.81, base)
	if err := store.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	const writers = 20
	errs := make(chan error, writers)
	var wait sync.WaitGroup
	for i := 0; i < writers; i++ {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			update := item
			update.Longitude = 106.8
			if i%2 == 1 {
				update.Longitude = 106.81
			}
			update.UpdatedAt = base.Add(time.Duration(i+1) * time.Second)
			errs <- store.Update(ctx, update)
		}(i)
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	page, err := store.GeofenceEvents(ctx, fence.ID, GeofenceEventFilters{}, MaxGeofenceEventPageSize, "")
	if err != nil || len(page.Data) == 0 {
		t.Fatalf("events=%+v err=%v", page, err)
	}
	ascending := append([]GeofenceEvent(nil), page.Data...)
	for i, j := 0, len(ascending)-1; i < j; i, j = i+1, j-1 {
		ascending[i], ascending[j] = ascending[j], ascending[i]
	}
	if ascending[0].EventType != "enter" {
		t.Fatalf("first transition should enter from initial outside: %+v", ascending)
	}
	for i := 1; i < len(ascending); i++ {
		if ascending[i].EventType == ascending[i-1].EventType {
			t.Fatalf("contradictory consecutive transitions: %+v", ascending)
		}
		if !ascending[i].OccurredAt.After(ascending[i-1].OccurredAt) {
			t.Fatalf("transition timestamps must preserve serialized order: %+v", ascending)
		}
	}
	memberships, err := store.EntityGeofences(ctx, item.ID)
	if err != nil || len(memberships) != 1 {
		t.Fatalf("final membership=%+v err=%v", memberships, err)
	}
	wantState := "outside"
	if ascending[len(ascending)-1].EventType == "enter" {
		wantState = "inside"
	}
	if memberships[0].State != wantState {
		t.Fatalf("event tail state=%s membership=%s", wantState, memberships[0].State)
	}
}

func TestConcurrentDefinitionAndCoordinateUpdatesKeepFinalMembershipConsistent(t *testing.T) {
	ctx := context.Background()
	store, _ := openGeofenceTestStore(t)
	fence := circleDefinition("00000000-0000-4000-8000-000000000181", "Concurrent definition", true, 250)
	if err := store.CreateGeofence(ctx, fence); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC()
	item := membershipEntity("00000000-0000-4000-8000-000000000281", "Concurrent entity", -6.2, 106.81, base)
	if err := store.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	const writers = 16
	errs := make(chan error, writers)
	var wait sync.WaitGroup
	for i := 0; i < writers; i++ {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			if i%2 == 0 {
				update := item
				update.Longitude = 106.8
				update.UpdatedAt = base.Add(time.Duration(i+1) * time.Second)
				errs <- store.Update(ctx, update)
				return
			}
			definition, err := store.GetGeofence(ctx, fence.ID)
			if err != nil {
				errs <- err
				return
			}
			radius := 250.0
			if i%4 == 1 {
				radius = 2000
			}
			definition.Geometry.RadiusM = &radius
			definition.UpdatedAt = base.Add(time.Duration(i+1) * time.Second)
			errs <- store.UpdateGeofence(ctx, definition)
		}(i)
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	finalEntity, err := store.Get(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	finalFence, err := store.GetGeofence(ctx, fence.ID)
	if err != nil {
		t.Fatal(err)
	}
	memberships, err := store.EntityGeofences(ctx, item.ID)
	if err != nil || len(memberships) != 1 {
		t.Fatalf("final membership=%+v err=%v", memberships, err)
	}
	want := membershipState(finalFence, finalEntity.Latitude, finalEntity.Longitude)
	if memberships[0].State != want {
		t.Fatalf("membership=%s want=%s entity=%+v fence=%+v", memberships[0].State, want, finalEntity, finalFence)
	}
}

func TestGeofenceEventRowsRejectUpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	store, _ := openGeofenceTestStore(t)
	fence := circleDefinition("00000000-0000-4000-8000-000000000161", "Append only", true, 250)
	if err := store.CreateGeofence(ctx, fence); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC()
	item := membershipEntity("00000000-0000-4000-8000-000000000261", "Mover", -6.2, 106.81, base)
	if err := store.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	updateMembershipEntity(t, store, item.ID, item.Name, -6.2, 106.8, base.Add(time.Minute))
	if _, err := store.db.Exec(`UPDATE geofence_events SET event_type='exit' WHERE geofence_id=?`, fence.ID); err == nil {
		t.Fatal("event update unexpectedly succeeded")
	}
	if _, err := store.db.Exec(`DELETE FROM geofence_events WHERE geofence_id=?`, fence.ID); err == nil {
		t.Fatal("event deletion unexpectedly succeeded")
	}
	page, err := store.GeofenceEvents(ctx, fence.ID, GeofenceEventFilters{}, 10, "")
	if err != nil || len(page.Data) != 1 || page.Data[0].EventType != "enter" {
		t.Fatalf("history changed after rejected mutation: %+v err=%v", page, err)
	}
}

func TestGeofenceDeletePreservesEnterExitHistoryAndActualSchemaConstraints(t *testing.T) {
	ctx := context.Background()
	store, _ := openGeofenceTestStore(t)
	fence := circleDefinition("00000000-0000-4000-8000-000000000191", "Delete audit", true, 250)
	if err := store.CreateGeofence(ctx, fence); err != nil {
		t.Fatal(err)
	}

	// Verify the actual migrated SQLite FK metadata: only current membership
	// references geofences/entities, while historical events have no parent FK.
	for table, wantFKs := range map[string]int{"geofence_events": 0, "entity_geofence_memberships": 2} {
		rows, err := store.db.Query(`PRAGMA foreign_key_list(` + table + `)`)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		cascadeTargets := map[string]bool{}
		for rows.Next() {
			var id, seq int
			var parent, from, to, onUpdate, onDelete, match string
			if err := rows.Scan(&id, &seq, &parent, &from, &to, &onUpdate, &onDelete, &match); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			count++
			if onDelete == "CASCADE" {
				cascadeTargets[parent] = true
			}
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
		if count != wantFKs {
			t.Fatalf("%s foreign key count=%d want=%d", table, count, wantFKs)
		}
		if table == "entity_geofence_memberships" && (!cascadeTargets["entities"] || !cascadeTargets["geofences"]) {
			t.Fatalf("membership cascade targets=%v", cascadeTargets)
		}
	}

	base := time.Now().UTC()
	mover := membershipEntity("00000000-0000-4000-8000-000000000291", "Mover snapshot", -6.2, 106.81, base)
	remaining := membershipEntity("00000000-0000-4000-8000-000000000292", "Current member", -6.2, 106.8, base)
	for _, item := range []entity.Entity{mover, remaining} {
		if err := store.Create(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	updateMembershipEntity(t, store, mover.ID, mover.Name, -6.2, 106.8, base.Add(time.Minute))
	updateMembershipEntity(t, store, mover.ID, mover.Name, -6.2, 106.81, base.Add(2*time.Minute))

	before, err := store.GeofenceEvents(ctx, fence.ID, GeofenceEventFilters{}, 10, "")
	if err != nil || len(before.Data) != 2 || before.Data[0].EventType != "exit" || before.Data[1].EventType != "enter" {
		t.Fatalf("expected persisted ENTER and EXIT history, page=%+v err=%v", before, err)
	}
	if _, err := store.db.Exec(`UPDATE geofence_events SET event_type='enter' WHERE geofence_id=?`, fence.ID); err == nil {
		t.Fatal("event UPDATE was not rejected")
	}
	if _, err := store.db.Exec(`DELETE FROM geofence_events WHERE geofence_id=?`, fence.ID); err == nil {
		t.Fatal("event DELETE was not rejected")
	}

	if err := store.Delete(ctx, mover.ID); err != nil {
		t.Fatal(err)
	}
	afterEntityDelete, err := store.GeofenceEvents(ctx, fence.ID, GeofenceEventFilters{}, 10, "")
	if err != nil || fmt.Sprint(afterEntityDelete.Data) != fmt.Sprint(before.Data) {
		t.Fatalf("entity deletion changed event history: before=%+v after=%+v err=%v", before.Data, afterEntityDelete.Data, err)
	}

	if err := store.DeleteGeofence(ctx, fence.ID); err != nil {
		t.Fatalf("delete geofence with multiple events: %v", err)
	}
	if _, err := store.GetGeofence(ctx, fence.ID); !errors.Is(err, ErrGeofenceNotFound) {
		t.Fatalf("deleted definition lookup error=%v", err)
	}
	var memberships int
	if err := store.db.QueryRow(`SELECT count(*) FROM entity_geofence_memberships WHERE geofence_id=?`, fence.ID).Scan(&memberships); err != nil || memberships != 0 {
		t.Fatalf("membership count after geofence delete=%d err=%v", memberships, err)
	}
	afterGeofenceDelete, err := store.GeofenceEvents(ctx, fence.ID, GeofenceEventFilters{}, 10, "")
	if err != nil || fmt.Sprint(afterGeofenceDelete.Data) != fmt.Sprint(before.Data) {
		t.Fatalf("geofence deletion changed event history: before=%+v after=%+v err=%v", before.Data, afterGeofenceDelete.Data, err)
	}
	if err := store.DeleteGeofence(ctx, fence.ID); !errors.Is(err, ErrGeofenceNotFound) {
		t.Fatalf("repeated deletion error=%v", err)
	}

	noEvents := circleDefinition("00000000-0000-4000-8000-000000000192", "No history", true, 250)
	if err := store.CreateGeofence(ctx, noEvents); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteGeofence(ctx, noEvents.ID); err != nil {
		t.Fatalf("delete geofence without event history: %v", err)
	}
	if err := store.DeleteGeofence(ctx, noEvents.ID); !errors.Is(err, ErrGeofenceNotFound) {
		t.Fatalf("repeated empty-history deletion error=%v", err)
	}
}

func TestMembershipAndGeofenceEventsSurviveRepositoryRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "restart.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	fence := circleDefinition("00000000-0000-4000-8000-000000000171", "Restart", true, 250)
	if err := store.CreateGeofence(ctx, fence); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC()
	item := membershipEntity("00000000-0000-4000-8000-000000000271", "Restarted", -6.2, 106.8, base)
	if err := store.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	updateMembershipEntity(t, store, item.ID, item.Name, -6.2, 106.81, base.Add(time.Minute))
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	memberships, err := reopened.EntityGeofences(ctx, item.ID)
	if err != nil || len(memberships) != 1 || memberships[0].State != "outside" {
		t.Fatalf("reopened membership=%+v err=%v", memberships, err)
	}
	events, err := reopened.GeofenceEvents(ctx, fence.ID, GeofenceEventFilters{}, 10, "")
	if err != nil || len(events.Data) != 1 || events.Data[0].EventType != "exit" {
		t.Fatalf("reopened events=%+v err=%v", events, err)
	}
}

func TestGeofenceCRUDAndPersistence(t *testing.T) {
	ctx := context.Background()
	store, path := openGeofenceTestStore(t)
	first := circleDefinition("00000000-0000-4000-8000-000000000001", "  Depot  ", true, 250)
	if err := store.CreateGeofence(ctx, first); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetGeofence(ctx, first.ID)
	if err != nil || got.Name != "Depot" || got.Geometry.Type != "Circle" || *got.Geometry.RadiusM != 250 {
		t.Fatalf("get=%+v err=%v", got, err)
	}
	duplicate := circleDefinition("00000000-0000-4000-8000-000000000002", "depot", false, 500)
	if err := store.CreateGeofence(ctx, duplicate); !errors.Is(err, ErrGeofenceNameConflict) {
		t.Fatalf("duplicate create error=%v", err)
	}

	updated := circleDefinition(first.ID, "North Depot", false, 500)
	updated.CreatedAt = got.CreatedAt
	updated.UpdatedAt = got.UpdatedAt.Add(time.Hour)
	if err := store.UpdateGeofence(ctx, updated); err != nil {
		t.Fatal(err)
	}
	got, err = store.GetGeofence(ctx, first.ID)
	if err != nil || got.Name != "North Depot" || got.Enabled || *got.Geometry.RadiusM != 500 || !got.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("updated=%+v err=%v", got, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()
	got, err = reopened.GetGeofence(ctx, first.ID)
	if err != nil || got.Name != "North Depot" || got.Enabled {
		t.Fatalf("reopened=%+v err=%v", got, err)
	}
	if err := reopened.DeleteGeofence(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if err := reopened.DeleteGeofence(ctx, first.ID); !errors.Is(err, ErrGeofenceNotFound) {
		t.Fatalf("delete absent error=%v", err)
	}
}

func TestGeofencePaginationAndFilterBoundCursor(t *testing.T) {
	ctx := context.Background()
	store, _ := openGeofenceTestStore(t)
	for _, item := range []geofence.Definition{
		circleDefinition("00000000-0000-4000-8000-000000000001", "A", true, 10),
		circleDefinition("00000000-0000-4000-8000-000000000002", "B", false, 20),
		circleDefinition("00000000-0000-4000-8000-000000000003", "C", true, 30),
	} {
		if err := store.CreateGeofence(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	page, err := store.Geofences(ctx, GeofenceFilters{}, 2, "")
	if err != nil || len(page.Data) != 2 || page.Meta.NextCursor == "" {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	next, err := store.Geofences(ctx, GeofenceFilters{}, 2, page.Meta.NextCursor)
	if err != nil || len(next.Data) != 1 || next.Meta.NextCursor != "" {
		t.Fatalf("next=%+v err=%v", next, err)
	}
	enabled := true
	if _, err := store.Geofences(ctx, GeofenceFilters{Enabled: &enabled}, 2, page.Meta.NextCursor); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("filter-mismatched cursor error=%v", err)
	}
	filtered, err := store.Geofences(ctx, GeofenceFilters{Enabled: &enabled}, 10, "")
	if err != nil || len(filtered.Data) != 2 {
		t.Fatalf("filtered=%+v err=%v", filtered, err)
	}
	if _, err := store.Geofences(ctx, GeofenceFilters{}, 101, ""); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("oversized page error=%v", err)
	}
}

func TestGeofenceActiveLimitAppliesOnCreateAndEnable(t *testing.T) {
	ctx := context.Background()
	store, _ := openGeofenceTestStore(t)
	for i := 0; i < geofence.MaxActiveGeofences; i++ {
		item := circleDefinition(fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1), fmt.Sprintf("active-%03d", i), true, 100)
		if err := store.CreateGeofence(ctx, item); err != nil {
			t.Fatalf("create active %d: %v", i, err)
		}
	}
	overflow := circleDefinition("00000000-0000-4000-8000-000000000999", "overflow", true, 100)
	if err := store.CreateGeofence(ctx, overflow); !errors.Is(err, ErrGeofenceLimit) {
		t.Fatalf("active create overflow error=%v", err)
	}
	disabled := circleDefinition("00000000-0000-4000-8000-000000000998", "disabled", false, 100)
	if err := store.CreateGeofence(ctx, disabled); err != nil {
		t.Fatalf("create disabled definition at active cap: %v", err)
	}
	disabled.Enabled = true
	if err := store.UpdateGeofence(ctx, disabled); !errors.Is(err, ErrGeofenceLimit) {
		t.Fatalf("enabling beyond active cap error=%v", err)
	}
}

func TestGeofenceMigrationUpgradesPopulatedV1AndRerunsSafely(t *testing.T) {
	db := newMigrationDB(t)
	seedLegacy(t, db)
	before := entitySnapshot(t, db)
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if got := entitySnapshot(t, db); len(got) != len(before) || got[0] != before[0] || got[1] != before[1] {
		t.Fatalf("entity rows changed: before=%v after=%v", before, got)
	}
	if got := ledgerVersions(t, db); len(got) != 7 || got[3] != 4 || got[4] != 5 || got[5] != 6 || got[6] != 7 {
		t.Fatalf("migration ledger=%v", got)
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("rerun migration: %v", err)
	}
	if got := ledgerVersions(t, db); len(got) != 7 {
		t.Fatalf("migration rerun changed ledger=%v", got)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM geofences").Scan(&count); err != nil || count != 0 {
		t.Fatalf("geofence table count=%d err=%v", count, err)
	}
	var indexes int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='index' AND name='idx_geofences_enabled_created_at_id'").Scan(&indexes); err != nil || indexes != 1 {
		t.Fatalf("geofence index count=%d err=%v", indexes, err)
	}
}

func TestGeofenceMigrationFailureRollsBackSchemaAndLedger(t *testing.T) {
	db := newMigrationDB(t)
	seedLegacy(t, db)
	ctx := context.Background()
	base := fstest.MapFS{}
	for _, name := range []string{
		"001_create_entities.sql",
		"002_advanced_entity_management.sql",
		"003_entity_search_and_pagination.sql",
	} {
		body, err := fs.ReadFile(migrations, "migrations/"+name)
		if err != nil {
			t.Fatal(err)
		}
		base["migrations/"+name] = &fstest.MapFile{Data: body}
	}
	if err := MigrateFS(ctx, db, base); err != nil {
		t.Fatalf("apply V2.4 schema fixture: %v", err)
	}
	migrationBody, err := fs.ReadFile(migrations, "migrations/004_geofences.sql")
	if err != nil {
		t.Fatal(err)
	}
	broken := append(append([]byte(nil), migrationBody...), []byte("\nCREATE TABLE idx_geofences_enabled_created_at_id(id INTEGER);\n")...)
	withBroken := fstest.MapFS{}
	for key, file := range base {
		withBroken[key] = file
	}
	withBroken["migrations/004_geofences.sql"] = &fstest.MapFile{Data: broken}
	if err := MigrateFS(ctx, db, withBroken); err == nil || !strings.Contains(err.Error(), "apply migration 004") {
		t.Fatalf("expected injected migration failure, got %v", err)
	}
	var found int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='geofences'").Scan(&found); err != nil || found != 0 {
		t.Fatalf("failed migration left geofences table: found=%d err=%v", found, err)
	}
	if got := ledgerVersions(t, db); len(got) != 3 || got[2] != 3 {
		t.Fatalf("failed migration changed ledger: %v", got)
	}
}

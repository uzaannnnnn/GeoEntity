package monitoring

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"geoentity/backend/internal/backup"
	"geoentity/backend/internal/entity"
	"geoentity/backend/internal/repository"
)

func testService(t *testing.T) (*Service, *repository.SQLite, string) {
	t.Helper()
	ctx := context.Background()
	store, err := repository.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	e := entity.Entity{ID: "10000000-0000-4000-8000-000000000001", Name: "Sensor host", Type: "facility", Status: "active", Latitude: 1, Longitude: 2, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err = store.Create(ctx, e); err != nil {
		t.Fatal(err)
	}
	return New(store.DB()), store, e.ID
}

func TestCredentialProvisionStoresOnlyHashAndAuthenticates(t *testing.T) {
	s, store, entityID := testService(t)
	d, token, err := s.CreateCredential(context.Background(), entityID, "test device", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(token, "gdev_") {
		t.Fatal("credential prefix missing")
	}
	var hash string
	if err = store.DB().QueryRow(`SELECT secret_hash FROM device_credentials WHERE device_id=?`, d.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if len(hash) != 64 || strings.Contains(token, hash) {
		t.Fatalf("bad stored credential hash: %q", hash)
	}
	id, gotEntity, credentialID, err := s.Authenticate(context.Background(), token)
	if err != nil || id != d.ID || gotEntity != entityID || credentialID == "" {
		t.Fatalf("authenticate=(%q,%q,%v)", id, gotEntity, err)
	}
	wrong := token[:len(token)-1] + "A"
	if _, _, _, err = s.Authenticate(context.Background(), wrong); err == nil {
		t.Fatal("wrong secret accepted")
	}
}

func TestIngestIdempotencyUnitAndHistory(t *testing.T) {
	s, store, entityID := testService(t)
	_, token, err := s.CreateCredential(context.Background(), entityID, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	device, entity, credentialID, err := s.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	in := Input{SensorID: "ambient_temperature", Value: 21.75, Unit: "Cel", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	first, dup, err := s.Ingest(context.Background(), device, entity, credentialID, "retry-key", in)
	if err != nil || dup {
		t.Fatalf("first ingest duplicate=%v err=%v", dup, err)
	}
	retry, dup, err := s.Ingest(context.Background(), device, entity, credentialID, "retry-key", in)
	if err != nil || !dup || retry.ID != first.ID {
		t.Fatalf("retry=%+v duplicate=%v err=%v", retry, dup, err)
	}
	in.Value = 22
	if _, _, err = s.Ingest(context.Background(), device, entity, credentialID, "retry-key", in); err != ErrConflict {
		t.Fatalf("changed payload err=%v", err)
	}
	in.Value = 21.75
	in.Unit = "V"
	if _, _, err = s.Ingest(context.Background(), device, entity, credentialID, "new-key", in); err == nil {
		t.Fatal("sensor unit change accepted")
	}
	var count int
	if err = store.DB().QueryRow(`SELECT count(*) FROM sensor_observations`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("observations=%d err=%v", count, err)
	}
}

func TestMigration007DefinesTelemetryOnlyAndAppendOnly(t *testing.T) {
	s, store, entityID := testService(t)
	rows, err := store.DB().Query(`SELECT name FROM sqlite_master WHERE type='table' AND name IN ('devices','device_credentials','device_sensors','sensor_observations','device_memberships','device_events') ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for rows.Next() {
		var n string
		if err = rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	rows.Close()
	if got := strings.Join(names, ","); got != "device_credentials,device_sensors,devices,sensor_observations" {
		t.Fatalf("unexpected telemetry schema tables: %s", got)
	}
	d, token, err := s.CreateCredential(context.Background(), entityID, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _, credentialID, err := s.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = s.Ingest(context.Background(), d.ID, entityID, credentialID, "k", Input{SensorID: "humidity", Value: 50, Unit: "%", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB().Exec(`INSERT INTO sensor_observations(id,entity_id,device_id,credential_id,idempotency_key,sensor_id,value,unit,observed_at,ingested_at,payload_sha256) VALUES('20000000-0000-4000-8000-000000000001',?,?,?,'bad-unit','humidity',1,'Cel',?,?,?)`, entityID, d.ID, credentialID, time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), strings.Repeat("a", 64)); err == nil {
		t.Fatal("database accepted an observation with mismatched sensor unit")
	}
	if _, err = store.DB().Exec(`UPDATE sensor_observations SET value=1`); err == nil {
		t.Fatal("observation update was allowed")
	}
	if _, err = store.DB().Exec(`DELETE FROM device_sensors`); err == nil {
		t.Fatal("sensor unit deletion was allowed")
	}
	var version int
	if err = store.DB().QueryRow(`SELECT max(version) FROM schema_migrations`).Scan(&version); err != nil || version != 7 {
		t.Fatalf("migration version=%d err=%v", version, err)
	}
}

func TestTimestampAndIdentifierValidation(t *testing.T) {
	s, _, entityID := testService(t)
	_, token, err := s.CreateCredential(context.Background(), entityID, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	device, entity, credentialID, err := s.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range []Input{{SensorID: "Bad", Value: 1, Unit: "Cel", ObservedAt: time.Now().UTC().Format(time.RFC3339)}, {SensorID: "ok", Value: 1, Unit: "unknown", ObservedAt: time.Now().UTC().Format(time.RFC3339)}, {SensorID: "ok", Value: 1, Unit: "Cel", ObservedAt: time.Now().Add(10 * time.Minute).Format(time.RFC3339)}} {
		if _, _, err = s.Ingest(context.Background(), device, entity, credentialID, "unique-key", in); err == nil {
			t.Fatalf("invalid input accepted: %+v", in)
		}
	}
}

func TestConcurrentRetriesAndEntityDeletionPreserveHistory(t *testing.T) {
	s, store, entityID := testService(t)
	d, token, err := s.CreateCredential(context.Background(), entityID, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _, credentialID, err := s.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	in := Input{SensorID: "temperature", Value: 10, Unit: "Cel", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	const n = 12
	var wg sync.WaitGroup
	var mu sync.Mutex
	ids := map[string]bool{}
	errs := []error{}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			o, _, e := s.Ingest(context.Background(), d.ID, entityID, credentialID, "concurrent-key", in)
			mu.Lock()
			defer mu.Unlock()
			if e != nil {
				errs = append(errs, e)
			} else {
				ids[o.ID] = true
			}
		}()
	}
	wg.Wait()
	if len(errs) > 0 || len(ids) != 1 {
		t.Fatalf("concurrent retry IDs=%v errors=%v", ids, errs)
	}
	if err = store.Delete(context.Background(), entityID); err != nil {
		t.Fatal(err)
	}
	var count int
	var status string
	var detached sql.NullString
	if err = store.DB().QueryRow(`SELECT count(*) FROM sensor_observations WHERE entity_id=?`, entityID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("retained history count=%d err=%v", count, err)
	}
	if err = store.DB().QueryRow(`SELECT status,entity_id FROM devices WHERE id=?`, d.ID).Scan(&status, &detached); err != nil || status != "detached" || detached.Valid {
		t.Fatalf("device state status=%s entity=%v err=%v", status, detached, err)
	}
	if _, _, _, err = s.Authenticate(context.Background(), token); err == nil {
		t.Fatal("deleted entity device credential still authorized")
	}
}

func TestCredentialRotationRevocationAndReassociation(t *testing.T) {
	s, store, entityID := testService(t)
	d, old, err := s.CreateCredential(context.Background(), entityID, "lifecycle", nil)
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := s.Rotate(context.Background(), d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = s.Authenticate(context.Background(), old); err == nil {
		t.Fatal("rotated credential remains valid")
	}
	if _, _, _, err = s.Authenticate(context.Background(), rotated); err != nil {
		t.Fatalf("rotated credential rejected: %v", err)
	}
	if err = s.Revoke(context.Background(), d.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = s.Authenticate(context.Background(), rotated); err == nil {
		t.Fatal("revoked credential remains valid")
	}
	if err = s.Detach(context.Background(), d.ID); err != nil {
		t.Fatal(err)
	}
	newEntity := entity.Entity{ID: "10000000-0000-4000-8000-000000000002", Name: "Second host", Type: "facility", Status: "active", Latitude: 3, Longitude: 4, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err = store.Create(context.Background(), newEntity); err != nil {
		t.Fatal(err)
	}
	reassociated, err := s.Reassociate(context.Background(), d.ID, newEntity.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, gotEntity, _, err := s.Authenticate(context.Background(), reassociated)
	if err != nil || gotEntity != newEntity.ID {
		t.Fatalf("reassociated credential scope=%q err=%v", gotEntity, err)
	}
	var active int
	if err = store.DB().QueryRow(`SELECT count(*) FROM device_credentials WHERE device_id=? AND revoked_at IS NULL`, d.ID).Scan(&active); err != nil || active != 1 {
		t.Fatalf("active credential count=%d err=%v", active, err)
	}
	var audits int
	if err = store.DB().QueryRow(`SELECT count(*) FROM admin_audit_events WHERE resource_type='device' AND resource_id=?`, d.ID).Scan(&audits); err != nil || audits < 4 {
		t.Fatalf("lifecycle audit count=%d err=%v", audits, err)
	}
}

func TestHistoryIndex(t *testing.T) {
	_, store, _ := testService(t)
	for _, tc := range []struct {
		name, sensor, cursorTime, cursorID, wantIndex string
	}{
		{name: "all sensors", wantIndex: "idx_sensor_observations_entity_time"},
		{name: "sensor-filtered cursor page", sensor: "temperature", cursorTime: "2026-01-02T00:00:00Z", cursorID: "cursor", wantIndex: "idx_sensor_observations_entity_sensor_time"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query, args := HistoryQuery("entity", tc.sensor, "2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z", tc.cursorTime, tc.cursorID, 50)
			rows, err := store.DB().Query(`EXPLAIN QUERY PLAN `+query, args...)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			plan := ""
			for rows.Next() {
				var id, parent, unused int
				var detail string
				if err = rows.Scan(&id, &parent, &unused, &detail); err != nil {
					t.Fatal(err)
				}
				plan += detail
			}
			if err = rows.Err(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(plan, tc.wantIndex) {
				t.Fatalf("production history query did not use %s: %s", tc.wantIndex, plan)
			}
		})
	}
}

func BenchmarkSyntheticIngest10K(b *testing.B) {
	for run := 0; run < b.N; run++ {
		ctx := context.Background()
		dbPath := filepath.Join(b.TempDir(), "synthetic.db")
		store, err := repository.Open(ctx, dbPath)
		if err != nil {
			b.Fatal(err)
		}
		e := entity.Entity{ID: "10000000-0000-4000-8000-000000000101", Name: "Synthetic host", Type: "facility", Status: "active", Latitude: 1, Longitude: 2, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
		if err = store.Create(ctx, e); err != nil {
			b.Fatal(err)
		}
		s := New(store.DB())
		_, token, err := s.CreateCredential(ctx, e.ID, "synthetic", nil)
		if err != nil {
			b.Fatal(err)
		}
		device, entityID, credentialID, err := s.Authenticate(ctx, token)
		if err != nil {
			b.Fatal(err)
		}
		input := Input{SensorID: "temperature", Value: 21.5, Unit: "Cel", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		start := time.Now()
		for i := 0; i < 10000; i++ {
			if _, _, err = s.Ingest(ctx, device, entityID, credentialID, "synthetic-"+strconv.Itoa(i), input); err != nil {
				b.Fatal(err)
			}
		}
		ingest := time.Since(start)
		qStart := time.Now()
		rows, err := store.DB().Query(`SELECT id FROM sensor_observations WHERE entity_id=? AND observed_at>=? AND observed_at<=? ORDER BY observed_at DESC,id DESC LIMIT 50`, entityID, time.Now().UTC().Add(-24*time.Hour).Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			b.Fatal(err)
		}
		historyCount := 0
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				b.Fatal(err)
			}
			historyCount++
		}
		rows.Close()
		if err != nil {
			b.Fatal(err)
		}
		queryLatency := time.Since(qStart)
		if historyCount != 50 {
			b.Fatalf("synthetic history page count=%d", historyCount)
		}
		dupStart := time.Now()
		if _, duplicate, e := s.Ingest(ctx, device, entityID, credentialID, "synthetic-9999", input); e != nil || !duplicate {
			b.Fatalf("duplicate retry duplicate=%v err=%v", duplicate, e)
		}
		duplicateLatency := time.Since(dupStart)
		backupDir := filepath.Join(b.TempDir(), "backups")
		backupStart := time.Now()
		manifest, backupErr := backup.Create(ctx, store.DB(), backupDir, 1)
		if backupErr != nil {
			b.Fatal(backupErr)
		}
		backupDuration := time.Since(backupStart)
		backupSize := manifest.SizeBytes
		restoreStart := time.Now()
		if backupErr = backup.Restore(ctx, filepath.Join(b.TempDir(), "restore.sqlite"), filepath.Join(backupDir, manifest.DatabaseFile+".manifest.json")); backupErr != nil {
			b.Fatal(backupErr)
		}
		restoreDuration := time.Since(restoreStart)
		if err = store.Close(); err != nil {
			b.Fatal(err)
		}
		info, err := os.Stat(dbPath)
		if err != nil {
			b.Fatal(err)
		}
		b.ReportMetric(float64(ingest.Microseconds())/10000, "us/observation")
		b.ReportMetric(float64(queryLatency.Microseconds()), "history_query_us")
		b.ReportMetric(float64(info.Size()), "db_bytes")
		b.ReportMetric(float64(duplicateLatency.Microseconds()), "duplicate_retry_us")
		b.ReportMetric(float64(backupDuration.Milliseconds()), "backup_ms")
		b.ReportMetric(float64(backupSize), "backup_bytes")
		b.ReportMetric(float64(restoreDuration.Milliseconds()), "restore_ms")
	}
}

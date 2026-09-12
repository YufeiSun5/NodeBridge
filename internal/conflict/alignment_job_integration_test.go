package conflict_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/alignment"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
)

func verifyPreparedSnapshot(t *testing.T, ctx context.Context, db *sql.DB, database string) {
	t.Helper()
	if err := alignment.CheckPendingJobs(ctx, db); err != nil {
		t.Fatal("empty job table blocks Agent", err)
	}
	for _, mode := range []string{"success", "lost_receipt", "receipt_failure", "source_receipt_failure"} {
		t.Run("prepared_"+mode, func(t *testing.T) {
			source, target := "job_src_"+mode, "job_dst_"+mode
			for _, query := range []string{"CREATE TABLE " + source + " (id BIGINT PRIMARY KEY, value BIGINT) ENGINE=InnoDB", "CREATE TABLE " + target + " (id BIGINT PRIMARY KEY, value BIGINT) ENGINE=InnoDB", "INSERT INTO " + source + " VALUES (1,50),(2,60)"} {
				if _, err := db.ExecContext(ctx, query); err != nil {
					t.Fatal(err)
				}
			}
			left, err := alignment.Observe(ctx, db, "edge", database, source)
			if err != nil {
				t.Fatal(err)
			}
			right, err := alignment.Observe(ctx, db, "server", database, target)
			if err != nil {
				t.Fatal(err)
			}
			rule := rules.SyncRule{ID: mode, DatabaseName: database, TableName: source, TargetDatabaseName: database, TargetTableName: target, PrimaryKeys: []string{"id"}, DeleteMode: rules.DeleteHard, InitialAlignment: rules.InitialAlignment{Policy: rules.AlignmentManual}}
			plan, err := alignment.BuildPlan(rule, left, right, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			for _, node := range []string{"edge", "server"} {
				if _, err := alignment.PrepareSnapshotJob(ctx, db, plan, rule, node, true); err != nil {
					t.Fatal(err)
				}
				if _, err := alignment.PrepareSnapshotJob(ctx, db, plan, rule, node, true); err == nil {
					t.Fatal("existing job overwritten")
				}
			}
			otherPlan, err := alignment.BuildPlan(rule, left, right, time.Now().Add(time.Microsecond))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := alignment.PrepareSnapshotJob(ctx, db, otherPlan, rule, "server", true); err == nil {
				t.Fatal("second plan displaced active table fence")
			}
			if mode == "receipt_failure" || mode == "source_receipt_failure" {
				phase := alignment.JobTargetCommitted
				if mode == "source_receipt_failure" {
					phase = alignment.JobTargetConfirmed
				}
				query := "CREATE TRIGGER owned_alignment_receipt_fail BEFORE UPDATE ON sync_alignment_job FOR EACH ROW BEGIN IF NEW.phase='" + phase + "' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='owned alignment receipt failure'; END IF; END"
				if _, err := db.ExecContext(ctx, query); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if _, err := db.ExecContext(ctx, "DROP TRIGGER owned_alignment_receipt_fail"); err != nil {
						t.Error(err)
					}
				}()
			}
			receiver, err := alignment.NewPreparedSnapshotReceiver(ctx, db, plan, rule, "server", true)
			if err != nil {
				t.Fatal(err)
			}
			defer receiver.Close()
			result, exportErr := alignment.ExportPreparedSnapshot(ctx, db, plan, rule, "edge", true, func(ctx context.Context, frame alignment.SnapshotFrame) (alignment.CopyResult, error) {
				receipt, err := receiver.Accept(ctx, frame)
				if frame.End != nil && err == nil && mode == "lost_receipt" {
					return alignment.CopyResult{}, errors.New("owned disconnected after commit")
				}
				return receipt, err
			})
			receiver.Close()
			if mode == "success" && (exportErr != nil || result.Rows != 2) {
				t.Fatal("confirmed transfer failed", result, exportErr)
			}
			if mode != "success" && exportErr == nil {
				t.Fatal("failure reported success")
			}
			if mode == "receipt_failure" && !strings.Contains(exportErr.Error(), "1644") {
				t.Fatal("receipt failure not exercised", exportErr)
			}
			targetJob, err := alignment.ReadSnapshotJob(ctx, db, plan, "server")
			if err != nil {
				t.Fatal(err)
			}
			sourceJob, err := alignment.ReadSnapshotJob(ctx, db, plan, "edge")
			if err != nil {
				t.Fatal(err)
			}
			wantRows := 2
			wantTarget, wantSource := alignment.JobTargetCommitted, alignment.JobSourceReady
			if mode == "success" {
				wantSource = alignment.JobTargetConfirmed
			}
			if mode == "receipt_failure" {
				wantTarget, wantRows = alignment.JobPrepared, 0
			}
			var count int
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+target).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if targetJob.Phase != wantTarget || sourceJob.Phase != wantSource || count != wantRows {
				t.Fatalf("non-atomic job/data: target=%+v source=%+v rows=%d", targetJob, sourceJob, count)
			}
			if wantRows != 0 && (targetJob.Result == nil || targetJob.Result.Rows != 2 || targetJob.Result.PlanID != plan.ID) {
				t.Fatal("durable target receipt missing")
			}
			if mode == "lost_receipt" {
				bad := targetJob
				altered := *targetJob.Result
				altered.Rows++
				bad.Result = &altered
				if _, err := alignment.ReconcileSnapshotReceipt(ctx, db, plan, rule, "edge", bad, true); err == nil {
					t.Fatal("recovery accepted changed manifest")
				}
				for attempt := 0; attempt < 2; attempt++ {
					recovered, err := alignment.ReconcileSnapshotReceipt(ctx, db, plan, rule, "edge", targetJob, true)
					if err != nil || recovered.Phase != alignment.JobTargetConfirmed {
						t.Fatal("lost receipt reconciliation failed", recovered, err)
					}
				}
			}
			if err := alignment.CheckPendingJobs(ctx, db); err == nil || !strings.Contains(err.Error(), "alignment_cutover_pending") {
				t.Fatal("copy receipt incorrectly permits Agent start", err)
			}
		})
	}
	t.Log("PASS: durable per-table preparation, target receipt/data atomicity, source confirmation and lost-receipt inspection; all remain blocked before CDC cutover")
}

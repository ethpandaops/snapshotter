package snapshotter

import (
	"context"
	"fmt"
	"time"

	"github.com/ethpandaops/eth-snapshotter/internal/db"
	log "github.com/sirupsen/logrus"
)

// StartCleanupRoutine starts a goroutine for cleaning up old snapshots
func (s *SnapShotter) StartCleanupRoutine() {
	if !s.cfg.Global.Snapshots.Cleanup.Enabled {
		log.Info("Snapshot cleanup is disabled")
		return
	}

	keepCount := s.cfg.Global.Snapshots.Cleanup.KeepCount
	if keepCount <= 0 {
		keepCount = 3 // Default to keeping the 3 most recent snapshots
	}

	checkIntervalHours := s.cfg.Global.Snapshots.Cleanup.CheckIntervalHours
	if checkIntervalHours <= 0 {
		checkIntervalHours = 24 // Default to checking once per day
	}

	log.WithFields(log.Fields{
		"keep_count":           keepCount,
		"check_interval_hours": checkIntervalHours,
	}).Info("starting snapshot cleanup routine")

	go func() {
		for {
			err := s.cleanupSnapshots(keepCount)
			if err != nil {
				log.WithError(err).Error("failed to cleanup snapshots")
			}

			// Sleep until next check
			time.Sleep(time.Duration(checkIntervalHours) * time.Hour)
		}
	}()
}

// cleanupSnapshots deletes old snapshots, keeping the most recent 'keepCount' snapshots
// and any snapshots/targets that are marked as persisted
func (s *SnapShotter) cleanupSnapshots(keepCount int) error {
	log.Info("running snapshot cleanup")

	// Get all successful snapshots that have not been deleted
	runs, err := s.db.GetSuccessfulRunsForCleanup()
	if err != nil {
		return fmt.Errorf("failed to get snapshots for cleanup: %w", err)
	}

	// First, exclude snapshots that are persisted at the run level
	var nonPersistedRuns []db.SnapshotRun
	var persistedRuns []db.SnapshotRun
	for _, run := range runs {
		if run.Persisted {
			persistedRuns = append(persistedRuns, run)
		} else {
			nonPersistedRuns = append(nonPersistedRuns, run)
		}
	}

	log.WithFields(log.Fields{
		"total_snapshots":    len(runs),
		"persisted_runs":     len(persistedRuns),
		"non_persisted_runs": len(nonPersistedRuns),
		"keep_count":         keepCount,
	}).Info("snapshot cleanup stats")

	if len(nonPersistedRuns) <= keepCount {
		log.WithFields(log.Fields{
			"non_persisted_count": len(nonPersistedRuns),
			"keep_count":          keepCount,
		}).Info("not enough non-persisted snapshots to cleanup")
		return nil
	}

	// The snapshots are ordered by block height DESC, so we keep the first 'keepCount' snapshots
	runsToProcess := nonPersistedRuns[keepCount:]

	log.WithFields(log.Fields{
		"total_snapshots": len(runs),
		"persisted_runs":  len(persistedRuns),
		"keep_count":      keepCount,
		"runs_to_process": len(runsToProcess),
	}).Info("found snapshots to process")

	// Now check target snapshots for each run to see if any individual targets are persisted
	for _, run := range runsToProcess {
		log.WithFields(log.Fields{
			"id":           run.ID,
			"block_height": run.BlockHeight,
			"start_time":   run.StartTime,
		}).Info("processing snapshot run for deletion")

		// Find which targets are persisted
		var persistedTargets []db.TargetSnapshot
		var nonPersistedTargets []db.TargetSnapshot

		for _, target := range run.TargetsSnapshot {
			// Skip targets that failed during snapshot creation
			if target.Status != "success" {
				continue
			}

			if target.Persisted {
				persistedTargets = append(persistedTargets, target)
			} else {
				nonPersistedTargets = append(nonPersistedTargets, target)
			}
		}

		log.WithFields(log.Fields{
			"run_id":                run.ID,
			"persisted_targets":     len(persistedTargets),
			"non_persisted_targets": len(nonPersistedTargets),
		}).Info("target snapshot stats")

		// Delete non-persisted targets
		for _, target := range nonPersistedTargets {
			log.WithFields(log.Fields{
				"id":            target.ID,
				"run_id":        run.ID,
				"target_alias":  target.Alias,
				"upload_prefix": target.UploadPrefix,
			}).Info("deleting target snapshot")

			// Delete the target snapshot files using S3 API
			if err := s.deleteTargetSnapshotFiles(target); err != nil {
				log.WithError(err).WithFields(log.Fields{
					"id":           target.ID,
					"target_alias": target.Alias,
				}).Error("failed to delete target snapshot files")
				continue
			}

			// Mark the target snapshot as deleted in the database
			if err := s.db.MarkTargetSnapshotAsDeleted(target.ID); err != nil {
				log.WithError(err).WithField("id", target.ID).Error("failed to mark target snapshot as deleted in database")
				continue
			}

			log.WithFields(log.Fields{
				"id":           target.ID,
				"target_alias": target.Alias,
			}).Info("successfully deleted target snapshot")
		}

		// If all targets are now deleted or were already deleted, mark the run as deleted
		allTargetsDeleted := true
		for _, target := range run.TargetsSnapshot {
			if target.Status == "success" && !target.Deleted && !target.Persisted {
				allTargetsDeleted = false
				break
			}
		}

		if allTargetsDeleted {
			log.WithField("id", run.ID).Info("all targets are deleted or persisted, marking run as deleted")
			if err := s.db.MarkSnapshotRunAsDeleted(run.ID); err != nil {
				log.WithError(err).WithField("id", run.ID).Error("failed to mark snapshot run as deleted in database")
				continue
			}
		}
	}

	return nil
}

// deleteTargetSnapshotFiles deletes the snapshot files for a specific target snapshot
func (s *SnapShotter) deleteTargetSnapshotFiles(target db.TargetSnapshot) error {
	ctx := context.Background()

	// Get the bucket name from the S3 client
	bucketName := s.s3Client.GetBucketName()
	if bucketName == "" {
		log.WithFields(log.Fields{
			"s3_bucket_config": s.cfg.Global.Snapshots.S3.BucketName,
			"target_alias":     target.Alias,
		}).Warn("no bucket name configured, skipping deletion")
		return fmt.Errorf("bucket name not configured in S3 settings")
	}

	log.WithFields(log.Fields{
		"target_alias": target.Alias,
		"path":         target.UploadPrefix,
		"bucket":       bucketName,
	}).Info("deleting target snapshot files")

	if s.cfg.Global.Snapshots.DryRun {
		log.WithFields(log.Fields{
			"path":   target.UploadPrefix,
			"bucket": bucketName,
		}).Warn("DRY RUN: Would delete target snapshot files")
		return nil
	}

	// Log the S3 configuration being used
	log.WithFields(log.Fields{
		"bucket":       bucketName,
		"s3_endpoint":  s.s3Client.GetEndpoint(),
		"s3_region":    s.s3Client.GetRegion(),
		"target_alias": target.Alias,
		"prefix":       target.UploadPrefix,
	}).Info("s3 deletion configuration")

	// Delete the snapshot directory using the S3 client
	err := s.s3Client.DeleteDirectory(ctx, bucketName, target.UploadPrefix)
	if err != nil {
		return fmt.Errorf("failed to delete snapshot for %s: %w", target.Alias, err)
	}

	log.WithFields(log.Fields{
		"target_alias": target.Alias,
		"bucket":       bucketName,
		"prefix":       target.UploadPrefix,
	}).Info("successfully deleted target snapshot files")

	return nil
}

// Legacy method - kept for compatibility, now uses the new per-target approach
func (s *SnapShotter) deleteSnapshotFiles(run db.SnapshotRun) error {
	var firstErr error

	for _, target := range run.TargetsSnapshot {
		// Skip targets that failed during snapshot creation
		if target.Status != "success" {
			continue
		}

		// Skip targets that are persisted
		if target.Persisted {
			log.WithFields(log.Fields{
				"target_alias": target.Alias,
				"run_id":       run.ID,
			}).Info("skipping persisted target snapshot")
			continue
		}

		err := s.deleteTargetSnapshotFiles(target)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

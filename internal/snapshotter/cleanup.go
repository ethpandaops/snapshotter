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
func (s *SnapShotter) cleanupSnapshots(keepCount int) error {
	log.Info("running snapshot cleanup")

	// Get all successful snapshots that have not been deleted
	runs, err := s.db.GetSuccessfulRunsForCleanup()
	if err != nil {
		return fmt.Errorf("failed to get snapshots for cleanup: %w", err)
	}

	if len(runs) <= keepCount {
		log.WithFields(log.Fields{
			"snapshot_count": len(runs),
			"keep_count":     keepCount,
		}).Info("not enough snapshots to cleanup")
		return nil
	}

	// The snapshots are ordered by block height DESC, so we keep the first 'keepCount' snapshots
	snapshotsToDelete := runs[keepCount:]

	log.WithFields(log.Fields{
		"total_snapshots":     len(runs),
		"keep_count":          keepCount,
		"snapshots_to_delete": len(snapshotsToDelete),
	}).Info("found snapshots to delete")

	for _, run := range snapshotsToDelete {
		log.WithFields(log.Fields{
			"id":           run.ID,
			"block_height": run.BlockHeight,
			"start_time":   run.StartTime,
		}).Info("deleting snapshot")

		// Delete the snapshot files using S3 API
		if err := s.deleteSnapshotFiles(run); err != nil {
			log.WithError(err).WithField("id", run.ID).Error("failed to delete snapshot files")
			continue
		}

		// Mark the snapshot as deleted in the database
		if err := s.db.MarkSnapshotRunAsDeleted(run.ID); err != nil {
			log.WithError(err).WithField("id", run.ID).Error("failed to mark snapshot as deleted in database")
			continue
		}

		log.WithField("id", run.ID).Info("successfully deleted snapshot")
	}

	return nil
}

// deleteSnapshotFiles deletes the snapshot files for a given snapshot run using S3 API
func (s *SnapShotter) deleteSnapshotFiles(run db.SnapshotRun) error {
	ctx := context.Background()
	var firstErr error

	for _, target := range run.TargetsSnapshot {
		// Skip targets that failed during snapshot creation
		if target.Status != "success" {
			continue
		}

		// Get the bucket name from the S3 client
		bucketName := s.s3Client.GetBucketName()
		if bucketName == "" {
			log.WithFields(log.Fields{
				"s3_bucket_config": s.cfg.Global.Snapshots.S3.BucketName,
				"target_alias":     target.Alias,
			}).Warn("no bucket name configured, skipping deletion")
			if firstErr == nil {
				firstErr = fmt.Errorf("bucket name not configured in S3 settings")
			}
			continue
		}

		log.WithFields(log.Fields{
			"target_alias": target.Alias,
			"path":         target.UploadPrefix,
			"bucket":       bucketName,
		}).Info("deleting snapshot files")

		if s.cfg.Global.Snapshots.DryRun {
			log.WithFields(log.Fields{
				"path":   target.UploadPrefix,
				"bucket": bucketName,
			}).Warn("DRY RUN: Would delete snapshot files")
			continue
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
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to delete snapshot for %s: %w", target.Alias, err)
			}
			log.WithError(err).WithFields(log.Fields{
				"target_alias": target.Alias,
				"bucket":       bucketName,
				"prefix":       target.UploadPrefix,
			}).Error("failed to delete snapshot files using S3 API")
			continue
		}

		log.WithFields(log.Fields{
			"target_alias": target.Alias,
			"bucket":       bucketName,
			"prefix":       target.UploadPrefix,
		}).Info("successfully deleted snapshot files")
	}

	return firstErr
}

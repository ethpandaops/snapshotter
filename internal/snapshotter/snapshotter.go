package snapshotter

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	sshClient "github.com/ethpandaops/eth-snapshotter/internal/clients/ssh"
	"github.com/ethpandaops/eth-snapshotter/internal/config"
	"github.com/ethpandaops/eth-snapshotter/internal/db"
	"github.com/ethpandaops/eth-snapshotter/internal/types"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type SnapShotter struct {
	cfg        *config.Config
	status     *types.SnapshotterStatus
	sshTargets []*sshTarget
	db         *db.DB
}

type sshTarget struct {
	client *sshClient.SSHClient
	cfg    *config.SSHTargetConfig
}

func Init(cfg *config.Config) (*SnapShotter, error) {
	log.WithFields(log.Fields{
		"check_interval_seconds": cfg.Global.Snapshots.CheckIntervalSeconds,
		"block_interval":         cfg.Global.Snapshots.BlockInterval,
		"run_once":               cfg.Global.Snapshots.RunOnce,
	}).Info("snapshot config")

	dbPath := cfg.Global.Database.Path
	if dbPath == "" {
		dbPath = "snapshots.db"
	}

	// Ensure directory exists
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := db.NewDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	ss := SnapShotter{
		cfg: cfg,
		status: &types.SnapshotterStatus{
			BlockInterval: uint64(cfg.Global.Snapshots.BlockInterval),
		},
		db: db,
	}

	sshTargets := make([]*sshTarget, len(cfg.Targets.SSH))

	for i, t := range cfg.Targets.SSH {
		tt := t
		sshTargets[i] = &sshTarget{
			client: sshClient.NewSSHClient(
				cfg.Global.SSH.PrivateKeyPath,
				cfg.Global.SSH.PrivateKeyPassphrasePath,
				cfg.Global.SSH.KnownHostsPath,
				cfg.Global.SSH.InsecureIgnoreHostKey,
				cfg.Global.SSH.UseAgent,
				&cfg.Global.Snapshots.RClone,
				&cfg.Targets.SSH[i],
			),
			cfg: &tt,
		}
	}

	log.Info("starting")

	ss.initValidations()

	ss.sshTargets = sshTargets

	return &ss, nil
}

func (s *SnapShotter) GetStatus() *types.SnapshotterStatus {
	return s.status
}

func (s *SnapShotter) initValidations() {
	var wg sync.WaitGroup
	for _, t := range s.sshTargets {
		wg.Add(1)
		cl := t.client
		go func() {
			defer wg.Done()
			chain, err := cl.GetELChainID()
			if err != nil {
				log.WithError(err).Fatalf("could not get chain Id from %s", cl.TargetConfig.Alias)
			}
			if chain != s.cfg.Global.ChainID {
				log.Fatalf("chain id mismatch for host %s . got %s expected %s", cl.TargetConfig.Alias, chain, s.cfg.Global.ChainID)
			}
			log.WithFields(log.Fields{
				"node":    cl.TargetConfig.Alias,
				"chainID": chain,
			}).Info("got correct chain ID from target")
		}()
	}
	wg.Wait()
}

func (s *SnapShotter) VerifyTargetsAreSynced() (bool, uint64) {
	var wg sync.WaitGroup

	syncResults := make(chan bool, 3*len(s.sshTargets))
	blockResults := make(chan uint64, len(s.sshTargets))
	for _, t := range s.sshTargets {
		wg.Add(3)
		cl := t.client
		tt := t
		go func() {

			// CL sync status
			go func() {
				defer wg.Done()
				status, err := cl.GetSyncStatusCL()
				if err != nil {
					log.WithFields(log.Fields{
						"host": cl.TargetConfig.Alias,
						"err":  err,
					}).Warn("failed getting sync status")
					syncResults <- false
					return
				}
				log.WithFields(log.Fields{
					"host":          cl.TargetConfig.Alias,
					"is_syncing":    status.IsSyncing,
					"is_optimistic": status.IsOptimistic,
					"sync_distance": status.SyncDistance,
					"el_offline":    status.ElOffline,
				}).Debug("got EL sync status")

				if status.IsSyncing {
					log.WithFields(log.Fields{
						"alias": tt.cfg.Alias,
						"host":  cl.TargetConfig.Alias,
					}).Warn("CL is syncing")
					syncResults <- false
					return
				}
				if status.IsOptimistic {
					log.WithFields(log.Fields{
						"alias": tt.cfg.Alias,
						"host":  cl.TargetConfig.Alias,
					}).Warn("CL is running in optimistic mode")
					syncResults <- false
					return
				}
				if status.ElOffline {
					log.WithFields(log.Fields{
						"alias": tt.cfg.Alias,
						"host":  cl.TargetConfig.Alias,
					}).Warn("CL can't connect to the EL")
					syncResults <- false
					return
				}
				sd, _ := strconv.Atoi(status.SyncDistance)
				if sd > 1 {
					log.WithFields(log.Fields{
						"alias":         tt.cfg.Alias,
						"host":          cl.TargetConfig.Alias,
						"sync_distance": status.SyncDistance,
						"head_slot":     status.HeadSlot,
					}).Warn("CL sync distance is > 1")
					syncResults <- false
					return
				}
				syncResults <- true
			}()

			// EL sync status
			go func() {
				defer wg.Done()
				syncing, err := cl.GetSyncStatusEL()
				if err != nil {
					log.Error("failed getting EL sync status")
					syncResults <- false
					return
				}
				log.WithFields(log.Fields{
					"alias": tt.cfg.Alias,
					"host":  cl.TargetConfig.Alias,
					"sync":  syncing,
				}).Debug("got EL sync status")
				syncResults <- true
			}()

			// EL block
			go func() {
				defer wg.Done()
				elBlockNumberHex, err := cl.GetELBlockNumber()
				if err != nil {
					log.Error("failed getting EL block")
					syncResults <- false
					blockResults <- 0
					return
				}
				elBlockNumberDec, _ := hexutil.DecodeUint64(elBlockNumberHex)

				log.WithFields(log.Fields{
					"alias":        tt.cfg.Alias,
					"host":         cl.TargetConfig.Alias,
					"el_block_hex": elBlockNumberHex,
					"el_block_dec": elBlockNumberDec,
				}).Debug("got EL block number")
				if err != nil {
					log.Error("failed getting EL block number")
				}
				syncResults <- true
				blockResults <- elBlockNumberDec
			}()
		}()
	}
	wg.Wait()
	close(syncResults)
	close(blockResults)

	allSynced := true
	for result := range syncResults {
		if !result {
			allSynced = false
		}
	}

	sameBlocks, block := checkIfAllSameResults(blockResults)

	if !sameBlocks {
		allSynced = false
	}

	return allSynced, block
}

func checkIfAllSameResults(ch chan uint64) (bool, uint64) {
	var firstValue uint64
	isFirstValueSet := false

	for value := range ch {
		if !isFirstValueSet {
			firstValue = value
			isFirstValueSet = true
			continue
		}
		if value != firstValue {
			return false, 0
		}
	}

	return isFirstValueSet, firstValue
}

// Add function that receives a block number and returns how many blocks are left to the next snapshot
func (s *SnapShotter) BlocksLeftToNextSnapshot(blockNumber uint64) uint64 {
	b := uint64(s.cfg.Global.Snapshots.BlockInterval) - (blockNumber % uint64(s.cfg.Global.Snapshots.BlockInterval))
	if b == uint64(s.cfg.Global.Snapshots.BlockInterval) {
		return 0
	}
	return b
}

func (s *SnapShotter) StartPeriodicPolling() {
	ticker := time.NewTicker(time.Duration(s.cfg.Global.Snapshots.CheckIntervalSeconds) * time.Second)
	quit := make(chan struct{})

	for {
		select {
		case <-ticker.C:
			t1 := time.Now()
			allSynced, blockNumber := s.VerifyTargetsAreSynced()
			if allSynced {
				blocksLeft := s.BlocksLeftToNextSnapshot(blockNumber)
				log.WithFields(log.Fields{
					"block_current":  blockNumber,
					"block_next":     blockNumber + blocksLeft,
					"blocks_left":    blocksLeft,
					"block_interval": s.cfg.Global.Snapshots.BlockInterval,
					"eta":            time.Duration(blocksLeft) * 12 * time.Second,
					"took":           time.Since(t1),
				}).Info("all targets are synced")
				s.status.Lock()
				s.status.ProcessedBlockHeight = blockNumber
				s.status.NextSnapshotBlockHeight = blockNumber + blocksLeft
				s.status.Unlock()

				run, err := s.db.GetMostRecentRun()
				if err != nil {
					log.WithError(err).Error("failed to get most recent run")
				}

				// If the most recent run is far away from the current block number, we need to create a new snapshot
				lastRunIsTooOld := false
				if run != nil && blockNumber > run.BlockHeight+uint64(s.cfg.Global.Snapshots.BlockInterval) {
					lastRunIsTooOld = true
					log.WithFields(log.Fields{
						"run_id":            run.ID,
						"block_current":     blockNumber,
						"block_last_run":    run.BlockHeight,
						"should_have_block": run.BlockHeight + uint64(s.cfg.Global.Snapshots.BlockInterval),
					}).Warn("most recent run is too old")
				}

				if blocksLeft == 0 || lastRunIsTooOld {
					log.WithFields(log.Fields{
						"block":          blockNumber,
						"block_interval": s.cfg.Global.Snapshots.BlockInterval,
					}).Info("reached block to be snapshotted")
					if err := s.CreateSnapshot(); err != nil {
						log.WithError(err).Error("failed to create snapshot")
					}

					if s.cfg.Global.Snapshots.RunOnce {
						log.Info("snapshot.run_once is true. shutting down")
						os.Exit(0)
					}

					waitSecs := 60
					log.Infof("waiting %d seconds for next run", waitSecs)
					time.Sleep(time.Duration(waitSecs) * time.Second)
				}
			}

		case <-quit:
			ticker.Stop()
			return
		}
	}
}

func (s *SnapShotter) CreateSnapshot() error {
	s.status.Lock()
	if s.status.SnapshotInProgress {
		s.status.Unlock()
		return fmt.Errorf("there's already a snapshot in progress")
	}
	s.status.SnapshotInProgress = true
	s.status.Unlock()
	defer func() {
		s.status.Lock()
		s.status.SnapshotInProgress = false
		s.status.Unlock()
	}()

	// Create snapshot run record
	run, err := s.db.CreateSnapshotRun(s.status.ProcessedBlockHeight, s.cfg.Global.Snapshots.DryRun)
	if err != nil {
		log.WithError(err).Error("failed to create snapshot run record")
		return err
	}

	log.WithFields(log.Fields{
		"run_id":  run.ID,
		"block":   run.BlockHeight,
		"dry_run": run.DryRun,
	}).Info("starting snapshot")

	if err := s.PrepareForSnapshot(); err != nil {
		if errDB := s.db.UpdateSnapshotRunStatus(run.ID, "failed", err.Error()); errDB != nil {
			log.WithError(errDB).Error("failed to update snapshot run status")
		}
		return err
	}

	if err := s.UploadSnapshot(run.ID); err != nil {
		if errDB := s.db.UpdateSnapshotRunStatus(run.ID, "failed", err.Error()); errDB != nil {
			log.WithError(errDB).Error("failed to update snapshot run status")
		}
		log.WithError(err).Error("failed to upload snapshot data")
		return err
	}

	if err := s.PostSnapshotStart(); err != nil {
		if errDB := s.db.UpdateSnapshotRunStatus(run.ID, "failed", err.Error()); errDB != nil {
			log.WithError(errDB).Error("failed to update snapshot run status")
		}
		log.WithError(err).Error("failed to restore service after snapshot")
		return err
	}

	if s.cfg.Global.Snapshots.DryRun {
		log.WithFields(log.Fields{
			"run_id": run.ID,
		}).Warn("dry run mode enabled - waiting 60s to update run status")
		time.Sleep(60 * time.Second)
	}
	if err := s.db.UpdateSnapshotRunStatus(run.ID, "success", ""); err != nil {
		log.WithError(err).Error("failed to update snapshot run status")
	}
	return err
}

func (s *SnapShotter) PrepareForSnapshot() error {
	if s.cfg.Global.Snapshots.DryRun {
		log.Warn("dry run mode enabled - skipping snapshot preparation")
		return nil
	}

	// Stop snooper
	log.Info("stopping snooper container across targets")
	group := errgroup.Group{}
	for _, t := range s.sshTargets {
		cl := t.client
		group.Go(func() error {
			err := cl.StopSnooper()
			if err != nil {
				log.WithError(err).Errorf("could not stop snooper  %s", cl.TargetConfig.Alias)
				return err
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	log.Info("stopped snooper across targets")

	log.Info("waiting to start checking checking if all nodes are still on the same block ")
	time.Sleep(30 * time.Second)

	// Check if EL blocks are really all the same
	blockResults := make(chan uint64, len(s.sshTargets))
	var wg sync.WaitGroup
	for _, t := range s.sshTargets {
		cl := t.client
		wg.Add(1)
		go func() {
			defer wg.Done()
			elBlockNumberHex, err := cl.GetELBlockNumber()
			if err != nil {
				log.Error("failed getting EL block")
				blockResults <- 0
				return
			}
			elBlockNumberDec, _ := hexutil.DecodeUint64(elBlockNumberHex)

			log.WithFields(log.Fields{
				"host":         cl.TargetConfig.Alias,
				"el_block_hex": elBlockNumberHex,
				"el_block_dec": elBlockNumberDec,
			}).Debug("got EL block number")
			if err != nil {
				log.Error("failed getting EL block number")
			}
			blockResults <- elBlockNumberDec

		}()
	}
	wg.Wait()
	close(blockResults)

	sameBlocks, block := checkIfAllSameResults(blockResults)

	if !sameBlocks {
		err := fmt.Errorf("failed due to not all targets reporting the same block height %d", block)
		log.WithError(err).Errorf("block %d doesn't match across all targets", block)
		return err
	}

	log.WithField("block", block).Info("all target ELs are at the same block")

	// Dump block info to file
	log.Info("dumping snapshot metadata to files")
	group = errgroup.Group{}
	for _, t := range s.sshTargets {
		cl := t.client
		tt := t
		group.Go(func() error {
			err := cl.DumpExecutionRPCRequestToFile(`{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["latest",true],"id":1}`, tt.cfg.DataDir+"/_snapshot_eth_getBlockByNumber.json")
			if err != nil {
				log.WithError(err).Errorf("could not dump eth_getBlockByNumber to file %s", cl.TargetConfig.Alias)
				return err
			}
			err = cl.DumpExecutionRPCRequestToFile(`{"jsonrpc":"2.0","method":"web3_clientVersion","params":[],"id":1}`, tt.cfg.DataDir+"/_snapshot_web3_clientVersion.json")
			if err != nil {
				log.WithError(err).Errorf("could not dump web3_clientVersion to file %s", cl.TargetConfig.Alias)
				return err
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}

	// Stop EL
	log.Info("stopping EL container across targets")
	group = errgroup.Group{}
	for _, t := range s.sshTargets {
		cl := t.client
		group.Go(func() error {
			err := cl.StopEL()
			if err != nil {
				log.WithError(err).Errorf("could not stop EL %s", cl.TargetConfig.Alias)
				return err
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	log.Info("stopped EL across targets")

	return nil
}

func (s *SnapShotter) PostSnapshotStart() error {
	if s.cfg.Global.Snapshots.DryRun {
		log.Warn("dry run mode enabled - skipping post snapshot sequence")
		return nil
	}

	// Start snooper
	log.Info("starting snooper container across targets")
	group := errgroup.Group{}
	for _, t := range s.sshTargets {
		cl := t.client
		group.Go(func() error {
			err := cl.StartSnooper()
			if err != nil {
				log.WithError(err).Errorf("could not start snooper  %s", cl.TargetConfig.Alias)
				return err
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	log.Info("started snooper across targets")

	// Start EL
	log.Info("starting EL container across targets")
	group = errgroup.Group{}
	for _, t := range s.sshTargets {
		cl := t.client
		group.Go(func() error {
			err := cl.StartEL()
			if err != nil {
				log.WithError(err).Errorf("could not start EL  %s", cl.TargetConfig.Alias)
				return err
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	log.Info("started EL across targets")

	// Restart beacon
	log.Info("restarting beacon container across targets")
	group = errgroup.Group{}
	for _, t := range s.sshTargets {
		cl := t.client
		group.Go(func() error {
			err := cl.RestartBeacon()
			if err != nil {
				log.WithError(err).Errorf("could not restart beacon %s", cl.TargetConfig.Alias)
				return err
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	log.Info("restarted beacon across targets")
	return nil
}

func (s *SnapShotter) UploadSnapshot(runID int64) error {
	t1 := time.Now()
	log.Info("starting uploading data snapshots")
	group := errgroup.Group{}

	for _, t := range s.sshTargets {
		cl := t.client
		tt := t

		// Append the block number to the upload prefix
		uploadPrefix := fmt.Sprintf("%s/%d", tt.cfg.UploadPrefix, s.status.ProcessedBlockHeight)

		targetSnapshot, err := s.db.CreateTargetSnapshot(runID, tt.cfg.Alias, uploadPrefix, s.cfg.Global.Snapshots.DryRun)
		if err != nil {
			log.WithError(err).Error("failed to create target snapshot record")
			continue
		}

		if s.cfg.Global.Snapshots.DryRun {
			log.WithFields(log.Fields{
				"alias":         tt.cfg.Alias,
				"upload_prefix": uploadPrefix,
			}).Warn("dry run mode enabled - skipping snapshot upload and waiting 60s to mark as success")
			go func() {
				time.Sleep(60 * time.Second)
				if err := s.db.UpdateTargetSnapshotStatus(targetSnapshot.ID, "success", ""); err != nil {
					log.WithError(err).Error("failed to update target snapshot status")
				}
			}()
			continue
		}

		group.Go(func() error {
			err := cl.RCloneSyncLocalToRemote(tt.cfg.DataDir, uploadPrefix)
			if err != nil {
				if errDB := s.db.UpdateTargetSnapshotStatus(targetSnapshot.ID, "failed", err.Error()); errDB != nil {
					log.WithError(errDB).Error("failed to update target snapshot status")
				}
				log.WithError(err).Errorf("could not upload via rclone %s", cl.TargetConfig.Alias)
				return err
			}

			if err := s.db.UpdateTargetSnapshotStatus(targetSnapshot.ID, "success", ""); err != nil {
				log.WithError(err).Error("failed to update target snapshot status")
			}
			log.WithFields(log.Fields{
				"alias":       tt.cfg.Alias,
				"uploaded_to": uploadPrefix,
				"height":      s.status.ProcessedBlockHeight,
				"took":        time.Since(t1),
			}).Info("uploaded data snapshot")
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"took": time.Since(t1),
	}).Info("finished uploading all data snapshots")
	return nil
}

func (s *SnapShotter) GetDB() *db.DB {
	return s.db
}

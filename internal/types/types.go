package types

import "sync"

type SnapshotterStatus struct {
	BlockInterval           uint64 `json:"blockInterval"`
	ProcessedBlockHeight    uint64 `json:"processedBlockHeight"`
	NextSnapshotBlockHeight uint64 `json:"nextPeriodSnapshotBlockHeight"`
	SnapshotInProgress      bool   `json:"snapshotInProgress"`
	sync.Mutex
}

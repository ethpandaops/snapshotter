package snapshotter

import (
	"context"
	"strings"
	"testing"

	"github.com/ethpandaops/eth-snapshotter/internal/config"
	"github.com/ethpandaops/eth-snapshotter/internal/types"
)

// MockS3Client is a mock implementation of the S3 client for testing
type MockS3Client struct {
	bucketName    string
	rootPrefix    string
	uploadedFiles map[string]string
}

func (m *MockS3Client) Initialize() error {
	return nil
}

func (m *MockS3Client) GetBucketName() string {
	return m.bucketName
}

func (m *MockS3Client) GetEndpoint() string {
	return "test-endpoint"
}

func (m *MockS3Client) GetRegion() string {
	return "test-region"
}

func (m *MockS3Client) GetRootPrefix() string {
	if m.rootPrefix != "" {
		prefix := m.rootPrefix
		if !strings.HasSuffix(prefix, "/") {
			prefix = prefix + "/"
		}
		return prefix
	}
	return ""
}

func (m *MockS3Client) PutObject(ctx context.Context, bucket, key string, content []byte) error {
	if m.uploadedFiles == nil {
		m.uploadedFiles = make(map[string]string)
	}
	m.uploadedFiles[key] = string(content)
	return nil
}

func (m *MockS3Client) DeleteDirectory(ctx context.Context, bucket, prefix string) error {
	// Mock implementation - not needed for these tests
	return nil
}

func TestUpdateLatestFile(t *testing.T) {
	// Create a mock S3 client
	mockS3 := &MockS3Client{
		bucketName: "test-bucket",
	}

	// Create a snapshotter with mock S3 client
	ss := &SnapShotter{
		cfg: &config.Config{},
		status: &types.SnapshotterStatus{
			ProcessedBlockHeight: 12345,
		},
		s3Client: mockS3,
	}

	// Test updateLatestFile
	err := ss.updateLatestFile()
	if err != nil {
		t.Fatalf("updateLatestFile failed: %v", err)
	}

	// Verify the latest file was uploaded with correct content and key
	if content, exists := mockS3.uploadedFiles["latest"]; !exists {
		t.Error("latest file was not uploaded")
	} else if content != "12345" {
		t.Errorf("latest file content incorrect. Expected '12345', got '%s'", content)
	}
}

func TestUpdateLatestFileWithRootPrefix(t *testing.T) {
	// Create a mock S3 client with root prefix
	mockS3 := &MockS3Client{
		bucketName: "test-bucket",
		rootPrefix: "mainnet",
	}

	// Create a snapshotter with mock S3 client
	ss := &SnapShotter{
		cfg: &config.Config{},
		status: &types.SnapshotterStatus{
			ProcessedBlockHeight: 67890,
		},
		s3Client: mockS3,
	}

	// Test updateLatestFile
	err := ss.updateLatestFile()
	if err != nil {
		t.Fatalf("updateLatestFile failed: %v", err)
	}

	// Verify the latest file was uploaded with correct content and prefixed key
	expectedKey := "mainnet/latest"
	if content, exists := mockS3.uploadedFiles[expectedKey]; !exists {
		t.Errorf("latest file was not uploaded with expected key '%s'", expectedKey)
	} else if content != "67890" {
		t.Errorf("latest file content incorrect. Expected '67890', got '%s'", content)
	}

	// Verify the file was not uploaded with the unprefixed key
	if _, exists := mockS3.uploadedFiles["latest"]; exists {
		t.Error("latest file was uploaded with unprefixed key when root prefix was configured")
	}
}

func TestUpdateLatestFileDryRun(t *testing.T) {
	// Create a mock S3 client
	mockS3 := &MockS3Client{
		bucketName: "test-bucket",
	}

	// Create a snapshotter with dry run enabled
	cfg := &config.Config{}
	cfg.Global.Snapshots.DryRun = true

	ss := &SnapShotter{
		cfg: cfg,
		status: &types.SnapshotterStatus{
			ProcessedBlockHeight: 12345,
		},
		s3Client: mockS3,
	}

	// Test updateLatestFile in dry run mode
	err := ss.updateLatestFile()
	if err != nil {
		t.Fatalf("updateLatestFile failed in dry run: %v", err)
	}

	// Verify no files were uploaded in dry run mode
	if len(mockS3.uploadedFiles) > 0 {
		t.Error("files were uploaded in dry run mode")
	}
}

func TestUpdateLatestFileWithRootPrefixAlreadyHasSlash(t *testing.T) {
	// Create a mock S3 client with root prefix that already has a trailing slash
	mockS3 := &MockS3Client{
		bucketName: "test-bucket",
		rootPrefix: "chains/ethereum/",
	}

	// Create a snapshotter with mock S3 client
	ss := &SnapShotter{
		cfg: &config.Config{},
		status: &types.SnapshotterStatus{
			ProcessedBlockHeight: 11111,
		},
		s3Client: mockS3,
	}

	// Test updateLatestFile
	err := ss.updateLatestFile()
	if err != nil {
		t.Fatalf("updateLatestFile failed: %v", err)
	}

	// Verify the latest file was uploaded with correct content and prefixed key
	expectedKey := "chains/ethereum/latest"
	if content, exists := mockS3.uploadedFiles[expectedKey]; !exists {
		t.Errorf("latest file was not uploaded with expected key '%s'", expectedKey)
	} else if content != "11111" {
		t.Errorf("latest file content incorrect. Expected '11111', got '%s'", content)
	}

	// Verify no double slashes were created
	for key := range mockS3.uploadedFiles {
		if strings.Contains(key, "//") {
			t.Errorf("double slash found in key: %s", key)
		}
	}
}

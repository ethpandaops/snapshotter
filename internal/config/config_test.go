package config

import (
	"os"
	"testing"
)

func TestEnvironmentVariableExpansion(t *testing.T) {
	// Set environment variables for testing
	if err := os.Setenv("TEST_S3_ENDPOINT", "https://test-s3-endpoint.com"); err != nil {
		t.Fatalf("Failed to set TEST_S3_ENDPOINT: %v", err)
	}
	if err := os.Setenv("TEST_S3_BUCKET", "test-bucket"); err != nil {
		t.Fatalf("Failed to set TEST_S3_BUCKET: %v", err)
	}
	if err := os.Setenv("TEST_S3_REGION", "test-region"); err != nil {
		t.Fatalf("Failed to set TEST_S3_REGION: %v", err)
	}

	// Create a temporary config file
	tmpConfigContent := `
server:
  listen_addr: 0.0.0.0:5001
global:
  logging: warn
  chainID: "0x88bb0"
  ssh:
    private_key_path: $HOME/.ssh/id_rsa
    private_key_passphrase_path: "$HOME/.ssh/id_rsa.passphrase"
    known_hosts_path: $HOME/.ssh/known_hosts
    ignore_host_key: false
    use_agent: false
  database:
    path: $HOME/snapshots.db
  snapshots:
    check_interval_seconds: 1
    block_interval: 10
    run_once: false
    cleanup:
      enabled: true
      keep_count: 3
      check_interval_hours: 1
    s3:
      bucket_name: "$TEST_S3_BUCKET"
      region: "$TEST_S3_REGION"
      endpoint: $TEST_S3_ENDPOINT
    rclone:
      version: "1.65.2"
      env:
        RCLONE_CONFIG_MYS3_TYPE: s3
        RCLONE_CONFIG_MYS3_PROVIDER: DigitalOcean
        RCLONE_CONFIG_MYS3_ACL: public-read
        RCLONE_CONFIG_MYS3_ENDPOINT: $TEST_S3_ENDPOINT
targets:
  ssh:
    - alias: "geth"
      host: "127.0.0.1"
      user: "test"
      port: 22
      data_dir: $HOME/data
      upload_prefix: test/geth
`
	tmpConfigPath := "config_test.yaml"
	err := os.WriteFile(tmpConfigPath, []byte(tmpConfigContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpConfigPath); err != nil {
			t.Errorf("Failed to remove temporary config file: %v", err)
		}
	}()

	// Read the config
	cfg, err := ReadFromFile(tmpConfigPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	// Test S3 environment variable expansion
	home := os.Getenv("HOME")
	if cfg.Global.Snapshots.S3.Endpoint != "https://test-s3-endpoint.com" {
		t.Errorf("S3 endpoint not expanded correctly. Got %s, expected https://test-s3-endpoint.com", cfg.Global.Snapshots.S3.Endpoint)
	}
	if cfg.Global.Snapshots.S3.BucketName != "test-bucket" {
		t.Errorf("S3 bucket name not expanded correctly. Got %s, expected test-bucket", cfg.Global.Snapshots.S3.BucketName)
	}
	if cfg.Global.Snapshots.S3.Region != "test-region" {
		t.Errorf("S3 region not expanded correctly. Got %s, expected test-region", cfg.Global.Snapshots.S3.Region)
	}

	// Test SSH environment variable expansion
	expectedPrivateKeyPath := home + "/.ssh/id_rsa"
	if cfg.Global.SSH.PrivateKeyPath != expectedPrivateKeyPath {
		t.Errorf("SSH private key path not expanded correctly. Got %s, expected %s", cfg.Global.SSH.PrivateKeyPath, expectedPrivateKeyPath)
	}

	// Test database path expansion
	expectedDBPath := home + "/snapshots.db"
	if cfg.Global.Database.Path != expectedDBPath {
		t.Errorf("Database path not expanded correctly. Got %s, expected %s", cfg.Global.Database.Path, expectedDBPath)
	}

	// Test SSH target data_dir expansion
	expectedDataDir := home + "/data"
	if cfg.Targets.SSH[0].DataDir != expectedDataDir {
		t.Errorf("SSH target data_dir not expanded correctly. Got %s, expected %s", cfg.Targets.SSH[0].DataDir, expectedDataDir)
	}

	// Test RClone env variable expansion
	if cfg.Global.Snapshots.RClone.Env["RCLONE_CONFIG_MYS3_ENDPOINT"] != "https://test-s3-endpoint.com" {
		t.Errorf("RClone endpoint env var not expanded correctly. Got %s, expected https://test-s3-endpoint.com",
			cfg.Global.Snapshots.RClone.Env["RCLONE_CONFIG_MYS3_ENDPOINT"])
	}

	// Verify that S3 config got propagated to RClone env
	if cfg.Global.Snapshots.RClone.Env["RCLONE_CONFIG_MYS3_REGION"] != "test-region" {
		t.Errorf("S3 region not propagated to RClone env correctly. Got %s, expected test-region",
			cfg.Global.Snapshots.RClone.Env["RCLONE_CONFIG_MYS3_REGION"])
	}
	if cfg.Global.Snapshots.RClone.Env["RCLONE_CONFIG_MYS3_BUCKET_NAME"] != "test-bucket" {
		t.Errorf("S3 bucket not propagated to RClone env correctly. Got %s, expected test-bucket",
			cfg.Global.Snapshots.RClone.Env["RCLONE_CONFIG_MYS3_BUCKET_NAME"])
	}
}

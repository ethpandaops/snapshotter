package ssh

import (
	"strings"
	"testing"

	"github.com/ethpandaops/eth-snapshotter/internal/config"
)

// NewSSHClient log.Fatals on missing key/known_hosts files, so tests build the struct directly.
func testSSHClient() *SSHClient {
	return &SSHClient{
		TargetConfig: &config.SSHTargetConfig{
			Alias:   "erigon",
			DataDir: "/data/erigon",
		},
		RCloneConfig: &config.RCloneConfig{
			Env: map[string]string{
				"RCLONE_CONFIG_MYS3_TYPE":        "s3",
				"RCLONE_CONFIG_MYS3_BUCKET_NAME": "test-bucket",
				"RCLONE_CONFIG_MYS3_ACL":         "public-read",
			},
			Version:         "1.74.2",
			Entrypoint:      "/bin/sh",
			CommandTemplate: config.DefaultRCloneCommandTemplate,
		},
	}
}

func TestBuildRCloneDockerCmdTarTemplate(t *testing.T) {
	client := testSSHClient()

	cmd, err := client.buildRCloneDockerCmd("/data/erigon", config.DefaultRCloneCommandTemplate, rcloneCmdVars{
		DataDir:          "/data/erigon",
		UploadPathPrefix: "hoodi/erigon",
		BucketName:       "test-bucket",
		BlockNumber:      123456,
	})
	if err != nil {
		t.Fatalf("buildRCloneDockerCmd failed: %v", err)
	}

	wantPrefix := "docker run --rm -v /data/erigon:/data/erigon --entrypoint /bin/sh"
	if !strings.HasPrefix(cmd, wantPrefix) {
		t.Errorf("command should start with %q. Got: %q", wantPrefix, cmd[:min(len(cmd), 120)])
	}

	// Env vars must be emitted in sorted key order (deterministic commands).
	wantEnv := " -e RCLONE_CONFIG_MYS3_ACL=public-read -e RCLONE_CONFIG_MYS3_BUCKET_NAME=test-bucket -e RCLONE_CONFIG_MYS3_TYPE=s3"
	if !strings.Contains(cmd, wantEnv) {
		t.Errorf("command should contain sorted env flags %q. Got: %q", wantEnv, cmd)
	}

	if !strings.Contains(cmd, ` rclone/rclone:1.74.2 -ac "`) {
		t.Errorf("command should invoke the rclone image with the -ac payload. Got: %q", cmd)
	}

	if !strings.Contains(cmd, "set -o pipefail") {
		t.Errorf("snapshot payload must set pipefail so a dying tar can't commit a truncated snapshot. Got: %q", cmd)
	}

	if !strings.Contains(cmd, "mys3:/test-bucket/hoodi/erigon/123456/snapshot.tar.zst") {
		t.Errorf("command should upload snapshot.tar.zst to the block dir. Got: %q", cmd)
	}

	if !strings.Contains(cmd, "--exclude=./"+config.PreimagesOutDirName) {
		t.Errorf("rendered tar payload should exclude the preimages scratch dir. Got: %q", cmd)
	}
}

func TestBuildRCloneDockerCmdPreimagesUpload(t *testing.T) {
	client := testSSHClient()

	cmd, err := client.buildRCloneDockerCmd("/data/erigon", config.DefaultPreimagesUploadCmdTemplate, rcloneCmdVars{
		DataDir:          "/data/erigon",
		OutDir:           "/data/erigon/_snapshot_preimages",
		UploadPathPrefix: "hoodi/erigon",
		BucketName:       "test-bucket",
		BlockNumber:      123456,
	})
	if err != nil {
		t.Fatalf("buildRCloneDockerCmd failed: %v", err)
	}

	if !strings.Contains(cmd, "set -o pipefail") {
		t.Errorf("preimages upload payload must set pipefail so a dying producer can't commit a truncated artifact. Got: %q", cmd)
	}

	if !strings.Contains(cmd, "cd /data/erigon/_snapshot_preimages") {
		t.Errorf("preimages upload payload should cd into the scratch dir. Got: %q", cmd)
	}

	// meta must be the FIRST tar member so consumers can stream it.
	if !strings.Contains(cmd, "tar -I 'zstd -T64' -cf - preimages.meta.json framed.bin") {
		t.Errorf("preimages upload payload must tar meta first, then framed.bin. Got: %q", cmd)
	}

	if !strings.Contains(cmd, "mys3:/test-bucket/hoodi/erigon/123456/preimages.tar.zst") {
		t.Errorf("preimages upload payload should rcat to the block dir. Got: %q", cmd)
	}

	// A dying producer still leaves a committed partial object; the chain
	// must remove it so a published block dir never holds a corrupt artifact.
	if !strings.Contains(cmd, "|| ( rclone deletefile mys3:/test-bucket/hoodi/erigon/123456/preimages.tar.zst; exit 1 )") {
		t.Errorf("preimages upload payload must delete the partial artifact on pipe failure. Got: %q", cmd)
	}

	// The preimages upload must never advance the per-client `latest` pointer.
	if strings.Contains(cmd, "/latest") {
		t.Errorf("preimages upload payload must never touch the latest pointer. Got: %q", cmd)
	}
}

func TestRenderPreimagesExportCmd(t *testing.T) {
	client := testSSHClient()

	cmd, err := client.renderPreimagesExportCmd("ethpandaops/erigon:test", "/data/erigon")
	if err != nil {
		t.Fatalf("renderPreimagesExportCmd failed: %v", err)
	}

	want := `u="$(stat -c '%u:%g' /data/erigon)" && docker run --rm --user "$u" -v /data/erigon:/data/erigon ethpandaops/erigon:test snapshots export-preimages --datadir /data/erigon --out /data/erigon/_snapshot_preimages`
	if cmd != want {
		t.Errorf("rendered export command mismatch.\nGot:  %q\nWant: %q", cmd, want)
	}
}

func TestResolvePreimagesImagePinned(t *testing.T) {
	client := testSSHClient()
	client.TargetConfig.Preimages.Image = "erigontech/erigon:main-latest"

	// A pinned image must resolve without any remote call (no SSH available in tests).
	image, err := client.ResolvePreimagesImage()
	if err != nil {
		t.Fatalf("ResolvePreimagesImage failed: %v", err)
	}
	if image != "erigontech/erigon:main-latest" {
		t.Errorf("expected pinned image, got %q", image)
	}
}

func TestResolvePreimagesImageRequiresPinOrContainer(t *testing.T) {
	client := testSSHClient() // no pinned image, no execution container configured

	_, err := client.ResolvePreimagesImage()
	if err == nil || !strings.Contains(err.Error(), "no image pinned") {
		t.Errorf("expected a config error when neither a pinned image nor an execution container is set, got: %v", err)
	}
}

func TestCleanupPreimagesRefusesBadDataDir(t *testing.T) {
	client := testSSHClient()

	for _, bad := range []string{"", "/"} {
		err := client.CleanupPreimages(bad)
		if err == nil || !strings.Contains(err.Error(), "refusing to") {
			t.Errorf("CleanupPreimages(%q) should refuse to run, got: %v", bad, err)
		}
	}
}

func TestExportAndUploadPreimagesRefusesBadDataDir(t *testing.T) {
	client := testSSHClient()
	client.TargetConfig.Preimages.Image = "erigontech/erigon:main-latest"

	for _, bad := range []string{"", "/"} {
		err := client.ExportAndUploadPreimages(bad, "hoodi/erigon", 123456)
		if err == nil || !strings.Contains(err.Error(), "refusing to run") {
			t.Errorf("ExportAndUploadPreimages(%q) should refuse before any remote call, got: %v", bad, err)
		}
	}
}

func TestValidateStateRootsOutput(t *testing.T) {
	root := "0x1a2b3c4d5e6f70819a2b3c4d5e6f70819a2b3c4d5e6f70819a2b3c4d5e6f7081"
	otherRoot := "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

	cases := []struct {
		name    string
		out     string
		wantErr bool
	}{
		{"matching roots", root + "\n" + root + "\n", false},
		{"malformed roots missing 0x prefix", strings.ToUpper(root[2:]) + "\n" + strings.ToUpper(root[2:]) + "\n", true},
		{"case-insensitive match", root + "\n0x" + strings.ToUpper(root[2:]) + "\n", false},
		{"mismatch", root + "\n" + otherRoot + "\n", true},
		{"null value", root + "\nnull\n", true},
		{"empty output", "", true},
		{"single line", root + "\n", true},
		{"noise then valid pair", "sudo: unable to resolve host xyz\n" + root + "\n" + root + "\n", false},
		{"interleaved sudo noise", "sudo: unable to resolve host xyz\n" + root + "\nsudo: unable to resolve host xyz\n" + root + "\n", false},
		{"trailing noise", root + "\n" + root + "\npam_unix(sudo:session): session closed\n", false},
		{"three hex lines", root + "\n" + root + "\n" + root + "\n", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStateRootsOutput(tc.out)
			if tc.wantErr && err == nil {
				t.Errorf("expected error for output %q", tc.out)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for output %q: %v", tc.out, err)
			}
		})
	}
}

package ssh

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/ethpandaops/eth-snapshotter/internal/config"
	"github.com/ethpandaops/eth-snapshotter/internal/types"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

type SSHClient struct {
	Config          *ssh.ClientConfig
	TargetConfig    *config.SSHTargetConfig
	RCloneConfig    *config.RCloneConfig
	PreimagesConfig *config.PreimagesConfig
}

// SnapshotMetadata represents metadata about a snapshot
type SnapshotMetadata struct {
	DockerImage string            `json:"docker_image,omitempty"`
	Static      map[string]string `json:"static,omitempty"`
}

func NewSSHClient(privateKeyPath, privateKeyPassphrasePath, knowHostsPath string, ignoreHostKeyCheck bool, useAgent bool, rclone *config.RCloneConfig, preimages *config.PreimagesConfig, target *config.SSHTargetConfig) *SSHClient {

	var hostkeyCallback ssh.HostKeyCallback
	hostkeyCallback, err := knownhosts.New(knowHostsPath)
	if err != nil {
		log.WithError(err).Fatal("failed reading known SSH hosts file")
	}

	if ignoreHostKeyCheck {
		hostkeyCallback = ssh.InsecureIgnoreHostKey()
		log.Warn("ssh server host keys are not being checked. this can be dangerous, only enable this if you understand the consequences")
	}

	auths := []ssh.AuthMethod{}

	if useAgent {
		// Use agent socket
		conn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
		if err != nil {
			log.Fatal(err)
		}
		defer func() {
			if err := conn.Close(); err != nil {
				log.WithError(err).Warn("failed to close SSH agent connection")
			}
		}()
		ag := agent.NewClient(conn)
		auths = append(auths, ssh.PublicKeysCallback(ag.Signers))
	} else {
		// Use private key
		key, err := os.ReadFile(privateKeyPath)
		if err != nil {
			log.WithError(err).Fatal("unable to read private key")
		}
		var signer ssh.Signer
		if privateKeyPassphrasePath == "" {
			signer, err = ssh.ParsePrivateKey(key)
			if err != nil {
				log.WithError(err).Fatal("unable to parse private key")
			}
		} else {
			passphrase, err := os.ReadFile(privateKeyPassphrasePath)
			if err != nil {
				log.WithError(err).Fatal("unable to read private key passphase file")
			}
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, passphrase[:len(passphrase)-1])
			if err != nil {
				log.WithError(err).Fatal("unable to parse private key using passphrase")
			}
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}

	config := &ssh.ClientConfig{
		User:            target.User,
		Auth:            auths,
		HostKeyCallback: hostkeyCallback,
	}

	return &SSHClient{
		Config:          config,
		TargetConfig:    target,
		RCloneConfig:    rclone,
		PreimagesConfig: preimages,
	}
}

func (client *SSHClient) RunCommand(cmd string) (string, error) {
	connection, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", client.TargetConfig.Host, client.TargetConfig.Port), client.Config)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := connection.Close(); err != nil {
			log.WithError(err).Warn("failed to close SSH connection")
		}
	}()

	session, err := connection.NewSession()
	if err != nil {
		return "", err
	}
	defer func() {
		if err := session.Close(); err != nil {
			// Check if error is EOF, which is expected when the server already closed the connection
			if err.Error() == "EOF" {
				log.WithField("host", client.TargetConfig.Host).Debug("SSH session already closed by server (EOF)")
			} else {
				log.WithError(err).Warn("failed to close SSH session")
			}
		}
	}()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(output), err
	}

	return string(output), nil
}

func (client *SSHClient) GetSyncStatusCL() (*types.BeaconV1NodeSyncing, error) {
	out, err := client.RunCommand(fmt.Sprintf(`bash -ac "
		curl -s %s/eth/v1/node/syncing | jq -r ".data"
	"`, client.TargetConfig.Endpoints.Beacon))
	if err != nil {
		log.WithFields(log.Fields{
			"err":  err,
			"host": client.TargetConfig.Alias,
		}).Warn("failed getting CL sync status")
		return nil, err
	}

	var status types.BeaconV1NodeSyncing
	err = json.Unmarshal([]byte(out), &status)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (client *SSHClient) DumpLatestBlockToFile(filePath string) error {
	cmd := fmt.Sprintf(`
	curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["latest",true],"id":1}' %s |
	jq -r "." | sudo tee %s`, client.TargetConfig.Endpoints.Execution, filePath)
	out, err := client.RunCommand(cmd)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"filePath": filePath,
			"out":      out,
		}).Error("failed to dump latest block info to file")
		return err
	}
	return nil
}

func (client *SSHClient) DumpExecutionRPCRequestToFile(payload, filePath string) error {
	cmd := fmt.Sprintf(`
	curl -s -X POST -H "Content-Type: application/json" --data '%s' %s |
	jq -r "." | sudo tee %s`, payload, client.TargetConfig.Endpoints.Execution, filePath)
	out, err := client.RunCommand(cmd)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"filePath": filePath,
			"out":      out,
		}).Error("failed to dump latest block info to file")
		return err
	}
	return nil
}

func (client *SSHClient) GetSyncStatusEL() (bool, error) {
	out, err := client.RunCommand(fmt.Sprintf(`
		curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":1}' %s | jq -r ".result"
	`, client.TargetConfig.Endpoints.Execution))
	if err != nil {
		log.WithError(err).Warn("failed getting EL sync status")
	}
	isSyncing, err := strconv.ParseBool(strings.TrimSuffix(out, "\n"))
	if err != nil {
		isSyncing = true
		syncingResp := struct {
			StartingBlock string `json:"startingBlock"`
			CurrentBlock  string `json:"currentBlock"`
			HighestBlock  string `json:"highestBlock"`
		}{}
		err = json.Unmarshal([]byte(out), &syncingResp)
		if err != nil {
			log.WithError(err).WithField("output", out).Error("failed parsing EL sync status output")
		} else {
			log.WithFields(log.Fields{
				"startingBlock": syncingResp.StartingBlock,
				"currentBlock":  syncingResp.CurrentBlock,
				"highestBlock":  syncingResp.HighestBlock,
			}).Warn("EL is syncing")
		}
		return isSyncing, err
	}
	return isSyncing, nil
}

func (client *SSHClient) GetELBlockNumber() (string, error) {
	out, err := client.RunCommand(fmt.Sprintf(`
		curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' %s | jq -r ".result"
	`, client.TargetConfig.Endpoints.Execution))
	if err != nil {
		log.WithError(err).Warn("failed getting EL block")
		return "", err
	}
	return strings.TrimSuffix(out, "\n"), nil
}

func (client *SSHClient) GetELChainID() (string, error) {
	out, err := client.RunCommand(fmt.Sprintf(`
		curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' %s | jq -r ".result"
	`, client.TargetConfig.Endpoints.Execution))
	if err != nil {
		log.WithError(err).WithField("output", out).Warn("failed getting EL chain id")
		return "", err
	}
	return strings.TrimSuffix(out, "\n"), nil
}

func (client *SSHClient) StopDockerContainer(name string) error {
	return client.StopDockerContainerWithForce(name, false)
}

func (client *SSHClient) StopDockerContainerWithForce(name string, force bool) error {
	args := ""
	if force {
		args += "-t 0"
	}
	out, err := client.RunCommand(fmt.Sprintf(`docker stop %s "%s"`, args, name))
	log.WithFields(log.Fields{
		"host":      client.TargetConfig.Alias,
		"container": name,
	}).Debug("stopping docker container")
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"container": name,
			"output":    out,
		}).Warn("failed to stop container")
		return err
	}
	return nil
}

func (client *SSHClient) StartDockerContainer(name string) error {
	out, err := client.RunCommand(fmt.Sprintf(`docker start "%s"`, name))
	log.WithFields(log.Fields{
		"host":      client.TargetConfig.Alias,
		"container": name,
	}).Debug("starting docker container")
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"container": name,
			"output":    out,
		}).Warn("failed to start container")
		return err
	}
	return nil
}

func (client *SSHClient) StopSnooper() error {
	return client.StopDockerContainerWithForce(client.TargetConfig.DockerContainers.EngineSnooper, true)
}

func (client *SSHClient) StartSnooper() error {
	return client.StartDockerContainer(client.TargetConfig.DockerContainers.EngineSnooper)
}

func (client *SSHClient) StopEL() error {
	return client.StopDockerContainer(client.TargetConfig.DockerContainers.Execution)
}

func (client *SSHClient) StartEL() error {
	return client.StartDockerContainer(client.TargetConfig.DockerContainers.Execution)
}

func (client *SSHClient) RestartBeacon() error {
	err := client.StopDockerContainer(client.TargetConfig.DockerContainers.Beacon)
	if err != nil {
		return err
	}
	return client.StartDockerContainer(client.TargetConfig.DockerContainers.Beacon)
}

func (client *SSHClient) GetDockerContainerImage(containerName string) (string, error) {
	cmd := fmt.Sprintf(`docker inspect --format='{{.Config.Image}}' "%s"`, containerName)
	out, err := client.RunCommand(cmd)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"container": containerName,
			"output":    out,
		}).Warn("failed to get container image")
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (client *SSHClient) RCloneSyncLocalToRemote(srcDir, uploadPrefix string, blockNumber uint64) error {
	// Get Docker image information for metadata
	metadata := SnapshotMetadata{
		Static: client.TargetConfig.Metadata,
	}

	// Get the execution container image if available
	if client.TargetConfig.DockerContainers.Execution != "" {
		dockerImage, err := client.GetDockerContainerImage(client.TargetConfig.DockerContainers.Execution)
		if err == nil {
			metadata.DockerImage = dockerImage
		} else {
			log.WithError(err).Warn("failed to get execution container image for metadata")
		}
	}

	// Create metadata JSON file
	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		log.WithError(err).Error("failed to marshal snapshot metadata")
		return err
	}

	// Write metadata to file
	metadataFile := fmt.Sprintf("%s/_snapshot_metadata.json", srcDir)
	metadataCmd := fmt.Sprintf(`echo '%s' | sudo tee %s`, string(metadataJSON), metadataFile)
	_, err = client.RunCommand(metadataCmd)
	if err != nil {
		log.WithError(err).Error("failed to write snapshot metadata file")
		return err
	}

	// Get command template, using default if not specified
	cmdTemplate := client.RCloneConfig.CommandTemplate
	if cmdTemplate == "" {
		// If we don't have the template directly, use the default from config package
		// This fallback should rarely happen since we set defaults in config.ReadFromFile
		log.Debug("RClone command template not specified, using default from config package")
		cmdTemplate = config.GetDefaultRCloneConfig().CommandTemplate
	}

	cmd, err := client.buildRCloneDockerCmd(srcDir, cmdTemplate, rcloneCmdVars{
		DataDir:          srcDir,
		UploadPathPrefix: uploadPrefix,
		BucketName:       client.resolveBucketName(),
		BlockNumber:      blockNumber,
	})
	if err != nil {
		return err
	}

	out, err := client.RunCommand(cmd)
	if err != nil {
		log.WithError(err).WithField("output", out).Error("failed to rclone sync")
		return err
	}

	return nil
}

// rcloneCmdVars are the variables exposed to rclone command templates.
// OutDir is only set for the preimages upload template.
type rcloneCmdVars struct {
	DataDir          string
	OutDir           string
	UploadPathPrefix string
	BucketName       string
	BlockNumber      uint64
}

func (client *SSHClient) resolveBucketName() string {
	if client.RCloneConfig.Env != nil {
		if val, exists := client.RCloneConfig.Env["RCLONE_CONFIG_MYS3_BUCKET_NAME"]; exists && val != "" {
			return val
		}
	}
	log.Warn("Bucket name not found in RClone config environment variables, using default")
	return "ethpandaops-ethereum-node-snapshots"
}

// buildRCloneDockerCmd assembles the `docker run ... rclone/rclone:<version> <payload>`
// command, rendering cmdTemplate with vars; env keys are sorted for determinism.
func (client *SSHClient) buildRCloneDockerCmd(mountDir, cmdTemplate string, vars rcloneCmdVars) (string, error) {
	cmd := "docker run --rm" +
		" -v " + mountDir + ":" + mountDir

	// Use default entrypoint if not specified
	entrypoint := client.RCloneConfig.Entrypoint
	if entrypoint == "" {
		entrypoint = config.GetDefaultRCloneConfig().Entrypoint
	}
	cmd += " --entrypoint " + entrypoint

	// Add environment variables
	if client.RCloneConfig.Env != nil {
		keys := make([]string, 0, len(client.RCloneConfig.Env))
		for k := range client.RCloneConfig.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			cmd += fmt.Sprintf(" -e %s=%s", k, client.RCloneConfig.Env[k])
		}
	}

	tmpl, err := template.New("cmd").Parse(cmdTemplate)
	if err != nil {
		log.WithError(err).Error("failed to parse rclone cmd template")
		return "", err
	}

	var rcloneCmd bytes.Buffer
	if err := tmpl.Execute(&rcloneCmd, vars); err != nil {
		log.WithError(err).Error("failed to execute rclone cmd template")
		return "", err
	}

	// Use default version if not specified
	version := client.RCloneConfig.Version
	if version == "" {
		version = config.GetDefaultRCloneConfig().Version
	}

	return cmd + " rclone/rclone:" + version + " " + rcloneCmd.String(), nil
}

// preimagesOutDir is inside the datadir so the existing bind mounts cover it;
// the snapshot tar template excludes it.
func preimagesOutDir(dataDir string) string {
	return dataDir + "/" + config.PreimagesOutDirName
}

func (client *SSHClient) preimagesExportTemplate() string {
	if client.PreimagesConfig != nil && client.PreimagesConfig.ExportCmdTemplate != "" {
		return client.PreimagesConfig.ExportCmdTemplate
	}
	return config.DefaultPreimagesExportCmdTemplate
}

func (client *SSHClient) preimagesUploadTemplate() string {
	if client.PreimagesConfig != nil && client.PreimagesConfig.UploadCmdTemplate != "" {
		return client.PreimagesConfig.UploadCmdTemplate
	}
	return config.DefaultPreimagesUploadCmdTemplate
}

func (client *SSHClient) renderPreimagesExportCmd(image, dataDir string) (string, error) {
	tmpl, err := template.New("preimages-export").Parse(client.preimagesExportTemplate())
	if err != nil {
		log.WithError(err).Error("failed to parse preimages export cmd template")
		return "", err
	}

	cmdVars := struct {
		Image   string
		DataDir string
		OutDir  string
	}{
		Image:   image,
		DataDir: dataDir,
		OutDir:  preimagesOutDir(dataDir),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cmdVars); err != nil {
		log.WithError(err).Error("failed to execute preimages export cmd template")
		return "", err
	}
	return buf.String(), nil
}

// PullDockerImage pulls an image on the target host.
func (client *SSHClient) PullDockerImage(image string) error {
	out, err := client.RunCommand(fmt.Sprintf(`docker pull "%s"`, image))
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"host":   client.TargetConfig.Alias,
			"image":  image,
			"output": out,
		}).Warn("failed to pull docker image")
		return err
	}
	return nil
}

// ResolvePreimagesImage returns the image to run the preimage export with:
// the pinned image from the target config when set, otherwise the image of
// the execution container (docker inspect works on stopped containers, and
// reusing the EL's own image guarantees binary/datadir version compatibility).
func (client *SSHClient) ResolvePreimagesImage() (string, error) {
	if image := client.TargetConfig.Preimages.Image; image != "" {
		return image, nil
	}
	if client.TargetConfig.DockerContainers.Execution == "" {
		return "", fmt.Errorf("preimages: no image pinned and no execution container configured for %s", client.TargetConfig.Alias)
	}
	return client.GetDockerContainerImage(client.TargetConfig.DockerContainers.Execution)
}

// RunPreimagesExport runs `erigon snapshots export-preimages` in a throwaway
// container. The EL must be stopped so the exported state matches the snapshot.
func (client *SSHClient) RunPreimagesExport(image, dataDir string) error {
	cmd, err := client.renderPreimagesExportCmd(image, dataDir)
	if err != nil {
		return err
	}
	out, err := client.RunCommand(cmd)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"host":   client.TargetConfig.Alias,
			"image":  image,
			"output": out,
		}).Error("preimages export failed")
		return fmt.Errorf("preimages export failed: %w", err)
	}
	return nil
}

// isHexRoot reports whether s is a 0x-prefixed 32-byte hex string.
func isHexRoot(s string) bool {
	if len(s) != 66 || !strings.HasPrefix(s, "0x") {
		return false
	}
	_, err := hex.DecodeString(s[2:])
	return err == nil
}

// validateStateRootsOutput expects exactly two well-formed 32-byte hex roots
// in the output (preimages meta first, block dump second) and requires them to
// match. Roots are selected by shape, so sudo/PAM noise anywhere in the merged
// stdout+stderr is ignored.
func validateStateRootsOutput(out string) error {
	var roots []string
	for _, line := range strings.Split(out, "\n") {
		if trimmed := strings.TrimSpace(line); isHexRoot(trimmed) {
			roots = append(roots, trimmed)
		}
	}
	if len(roots) != 2 {
		return fmt.Errorf("expected exactly 2 state roots in verification output, got %d: %q", len(roots), out)
	}
	if !strings.EqualFold(roots[0], roots[1]) {
		return fmt.Errorf("state root mismatch: preimages meta has %s but snapshot block dump has %s", roots[0], roots[1])
	}
	return nil
}

// verifyPreimagesStateRoot requires the state root in preimages.meta.json to
// equal the one in the dumped _snapshot_eth_getBlockByNumber.json, catching
// commitment lag and wrong-datadir exports before upload. Block numbers are
// deliberately not compared: the S3 dir name can lag the frozen head.
func (client *SSHClient) verifyPreimagesStateRoot(dataDir string) error {
	metaPath := preimagesOutDir(dataDir) + "/preimages.meta.json"
	dumpPath := dataDir + "/_snapshot_eth_getBlockByNumber.json"
	cmd := fmt.Sprintf(`sudo cat "%s" | jq -r .stateRoot && sudo cat "%s" | jq -r .result.stateRoot`, metaPath, dumpPath)
	out, err := client.RunCommand(cmd)
	if err != nil {
		return fmt.Errorf("preimages state root verification failed: %w (output: %s)", err, out)
	}
	if err := validateStateRootsOutput(out); err != nil {
		return fmt.Errorf("preimages state root verification failed: %w", err)
	}
	return nil
}

// UploadPreimages uploads the export output as preimages.tar.zst to the block
// directory via the rclone container.
func (client *SSHClient) UploadPreimages(dataDir, uploadPrefix string, blockNumber uint64) error {
	cmd, err := client.buildRCloneDockerCmd(dataDir, client.preimagesUploadTemplate(), rcloneCmdVars{
		DataDir:          dataDir,
		OutDir:           preimagesOutDir(dataDir),
		UploadPathPrefix: uploadPrefix,
		BucketName:       client.resolveBucketName(),
		BlockNumber:      blockNumber,
	})
	if err != nil {
		return err
	}
	out, err := client.RunCommand(cmd)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"host":   client.TargetConfig.Alias,
			"output": out,
		}).Error("preimages upload failed")
		return fmt.Errorf("preimages upload failed: %w", err)
	}
	return nil
}

// CleanupPreimages removes the preimages scratch directory from the datadir.
func (client *SSHClient) CleanupPreimages(dataDir string) error {
	if dataDir == "" || dataDir == "/" {
		return fmt.Errorf("refusing to clean up preimages scratch dir for unsafe data dir %q", dataDir)
	}
	out, err := client.RunCommand(fmt.Sprintf(`sudo rm -rf "%s"`, preimagesOutDir(dataDir)))
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"host":   client.TargetConfig.Alias,
			"output": out,
		}).Warn("failed to clean up preimages scratch dir")
		return err
	}
	return nil
}

// ExportAndUploadPreimages exports, verifies the state root, then uploads.
// Scratch-dir cleanup always runs afterwards but never fails the flow.
func (client *SSHClient) ExportAndUploadPreimages(dataDir, uploadPrefix string, blockNumber uint64) error {
	if dataDir == "" || dataDir == "/" {
		return fmt.Errorf("preimages: refusing to run against unsafe data dir %q", dataDir)
	}

	image, err := client.ResolvePreimagesImage()
	if err != nil {
		return err
	}

	// Warn-only: export-preimages rewrites its outputs (erigontech/erigon#22645).
	if err := client.CleanupPreimages(dataDir); err != nil {
		log.WithError(err).Warnf("failed to pre-clean preimages scratch dir on %s (continuing)", client.TargetConfig.Alias)
	}
	defer func() {
		if err := client.CleanupPreimages(dataDir); err != nil {
			log.WithError(err).Warnf("failed to clean up preimages scratch dir on %s (snapshot tar excludes it)", client.TargetConfig.Alias)
		}
	}()

	t1 := time.Now()
	log.WithFields(log.Fields{
		"host":  client.TargetConfig.Alias,
		"image": image,
	}).Info("exporting state preimages")

	if err := client.RunPreimagesExport(image, dataDir); err != nil {
		return err
	}
	if err := client.verifyPreimagesStateRoot(dataDir); err != nil {
		return err
	}
	if err := client.UploadPreimages(dataDir, uploadPrefix, blockNumber); err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"host": client.TargetConfig.Alias,
		"took": time.Since(t1),
	}).Info("exported and uploaded state preimages")
	return nil
}

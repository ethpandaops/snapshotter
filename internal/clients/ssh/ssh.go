package ssh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"text/template"

	log "github.com/sirupsen/logrus"

	"github.com/ethpandaops/eth-snapshotter/internal/config"
	"github.com/ethpandaops/eth-snapshotter/internal/types"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

type SSHClient struct {
	Config       *ssh.ClientConfig
	TargetConfig *config.SSHTargetConfig
	RCloneConfig *config.RCloneConfig
}

// SnapshotMetadata represents metadata about a snapshot
type SnapshotMetadata struct {
	DockerImage string            `json:"docker_image,omitempty"`
	Static      map[string]string `json:"static,omitempty"`
}

func NewSSHClient(privateKeyPath, privateKeyPassphrasePath, knowHostsPath string, ignoreHostKeyCheck bool, useAgent bool, rclone *config.RCloneConfig, target *config.SSHTargetConfig) *SSHClient {

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
		Config:       config,
		TargetConfig: target,
		RCloneConfig: rclone,
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

	cmd := "docker run --rm" +
		" -v " + srcDir + ":" + srcDir

	// Use default entrypoint if not specified
	entrypoint := client.RCloneConfig.Entrypoint
	if entrypoint == "" {
		entrypoint = config.GetDefaultRCloneConfig().Entrypoint
	}
	cmd += " --entrypoint " + entrypoint

	// Add environment variables
	if client.RCloneConfig.Env != nil {
		for k, v := range client.RCloneConfig.Env {
			cmd += fmt.Sprintf(" -e %s=%s", k, v)
		}
	}

	// Get command template, using default if not specified
	cmdTemplate := client.RCloneConfig.CommandTemplate
	if cmdTemplate == "" {
		// If we don't have the template directly, use the default from config package
		// This fallback should rarely happen since we set defaults in config.ReadFromFile
		log.Debug("RClone command template not specified, using default from config package")
		cmdTemplate = config.GetDefaultRCloneConfig().CommandTemplate
	}

	tmpl, err := template.New("cmd").Parse(cmdTemplate)
	if err != nil {
		log.WithError(err).Error("failed to parse rclone cmd template")
		return err
	}

	// Get bucket name from RClone config environment variables
	// This is set in config.ReadFromFile from the s3 configuration
	bucketName := ""
	if client.RCloneConfig.Env != nil {
		if val, exists := client.RCloneConfig.Env["RCLONE_CONFIG_MYS3_BUCKET_NAME"]; exists && val != "" {
			bucketName = val
		}
	}

	// If not found in rclone env, use the default
	if bucketName == "" {
		log.Warn("Bucket name not found in RClone config environment variables, using default")
		// Use a default if no environment variable is set
		bucketName = "ethpandaops-ethereum-node-snapshots"
	}

	cmdVars := struct {
		DataDir          string
		UploadPathPrefix string
		BucketName       string
		BlockNumber      uint64
	}{
		DataDir:          srcDir,
		UploadPathPrefix: uploadPrefix,
		BucketName:       bucketName,
		BlockNumber:      blockNumber,
	}

	var rcloneCmd bytes.Buffer
	if err := tmpl.Execute(&rcloneCmd, cmdVars); err != nil {
		log.WithError(err).Error("failed to execute rclone cmd template")
		return err
	}

	// Use default version if not specified
	version := client.RCloneConfig.Version
	if version == "" {
		version = config.GetDefaultRCloneConfig().Version
	}

	cmd += " rclone/rclone:" + version + " " + rcloneCmd.String()
	out, err := client.RunCommand(cmd)
	if err != nil {
		log.WithError(err).WithField("output", out).Error("failed to rclone sync")
		return err
	}

	return nil
}

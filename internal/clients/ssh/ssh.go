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
	Host         string
	Port         int
	RCloneConfig *config.RCloneConfig
}

func NewSSHClient(user, host, privateKeyPath, privateKeyPassphrasePath, knowHostsPath string, port int, ignoreHostKeyCheck bool, useAgent bool, rclone *config.RCloneConfig) *SSHClient {

	var hostkeyCallback ssh.HostKeyCallback
	hostkeyCallback, err := knownhosts.New(os.ExpandEnv(knowHostsPath))
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
		defer conn.Close()
		ag := agent.NewClient(conn)
		auths = append(auths, ssh.PublicKeysCallback(ag.Signers))
	} else {
		// Use private key
		key, err := os.ReadFile(os.ExpandEnv(privateKeyPath))
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
			passphrase, err := os.ReadFile(os.ExpandEnv(privateKeyPassphrasePath))
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
		User:            user,
		Auth:            auths,
		HostKeyCallback: hostkeyCallback,
	}

	return &SSHClient{
		Config:       config,
		Host:         host,
		Port:         port,
		RCloneConfig: rclone,
	}
}

func (client *SSHClient) RunCommand(cmd string) (string, error) {
	connection, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", client.Host, client.Port), client.Config)
	if err != nil {
		return "", err
	}
	defer connection.Close()

	session, err := connection.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(output), err
	}

	return string(output), nil
}

func (client *SSHClient) GetSyncStatusCL() (*types.BeaconV1NodeSyncing, error) {
	out, err := client.RunCommand(`bash -ac "
		curl -s http://localhost:5052/eth/v1/node/syncing | jq -r ".data"
	"`)
	if err != nil {
		log.WithError(err).Warn("failed getting CL sync status")
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
	curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["latest",true],"id":1}' http://localhost:8545 |
	jq -r ".result" | sudo tee %s`, filePath)
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
	out, err := client.RunCommand(`
		curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":1}' http://localhost:8545 | jq -r ".result"
	`)
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
	out, err := client.RunCommand(`
		curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' http://localhost:8545 | jq -r ".result"
	`)
	if err != nil {
		log.WithError(err).Warn("failed getting EL block")
		return "", err
	}
	return strings.TrimSuffix(out, "\n"), nil
}

func (client *SSHClient) GetELChainID() (string, error) {
	out, err := client.RunCommand(`
		curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' http://localhost:8545 | jq -r ".result"
	`)
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
		"host":      client.Host,
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
		"host":      client.Host,
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
	return client.StopDockerContainerWithForce("snooper-engine", true)
}

func (client *SSHClient) StartSnooper() error {
	return client.StartDockerContainer("snooper-engine")
}

func (client *SSHClient) StopCL() error {
	return client.StopDockerContainer("beacon")
}

func (client *SSHClient) StartCL() error {
	return client.StartDockerContainer("beacon")
}

func (client *SSHClient) StopEL() error {
	return client.StopDockerContainer("execution")
}

func (client *SSHClient) StartEL() error {
	return client.StartDockerContainer("execution")
}

func (client *SSHClient) RCloneSyncLocalToRemote(srcDir, uploadPathPrefix string) error {
	cmd := "docker run --rm" +
		" -v " + srcDir + ":" + srcDir
	for k, v := range client.RCloneConfig.Env {
		cmd += fmt.Sprintf(" -e %s=%s", k, v)
	}

	tmpl, err := template.New("cmd").Parse(client.RCloneConfig.CommandTemplate)
	if err != nil {
		log.WithError(err).Error("failed to parse rclone cmd template")
		return err
	}

	cmdVars := struct {
		DataDir          string
		UploadPathPrefix string
	}{
		DataDir:          srcDir,
		UploadPathPrefix: uploadPathPrefix,
	}

	var rcloneCmd bytes.Buffer
	if err := tmpl.Execute(&rcloneCmd, cmdVars); err != nil {
		log.WithError(err).Error("failed to execute rclone cmd template")
		return err
	}

	cmd += " rclone/rclone:" + client.RCloneConfig.Version + " " + rcloneCmd.String()
	out, err := client.RunCommand(cmd)
	if err != nil {
		log.WithError(err).WithField("output", out).Error("failed to rclone sync")
		return err
	}

	return nil
}

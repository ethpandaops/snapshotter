package config

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Global struct {
		Logging string `yaml:"logging"`
		ChainID string `yaml:"chainID"`
		SSH     struct {
			PrivateKeyPath           string `yaml:"private_key_path"`
			PrivateKeyPassphrasePath string `yaml:"private_key_passphrase_path"`
			KnownHostsPath           string `yaml:"known_hosts_path"`
			InsecureIgnoreHostKey    bool   `yaml:"ignore_host_key"`
			UseAgent                 bool   `yaml:"use_agent"`
		} `yaml:"ssh"`
		Snapshots struct {
			CheckIntervalSeconds int          `yaml:"check_interval_seconds"`
			BlockInterval        int          `yaml:"block_interval"`
			RunOnce              bool         `yaml:"run_once"`
			RClone               RCloneConfig `yaml:"rclone"`
		} `yaml:"snapshots"`
		Database struct {
			Path string `yaml:"path"`
		} `yaml:"database"`
	} `yaml:"global"`
	Targets struct {
		SSH []SSHTargetConfig `yaml:"ssh"`
	} `yaml:"targets"`
}

type SSHTargetConfig struct {
	Alias            string `yaml:"alias"`
	Host             string `yaml:"host"`
	User             string `yaml:"user"`
	Port             int    `yaml:"port"`
	DataDir          string `yaml:"data_dir"`
	UploadPrefix     string `yaml:"upload_prefix"`
	DockerContainers struct {
		EngineSnooper string `yaml:"engine_snooper"`
		Execution     string `yaml:"execution"`
	} `yaml:"docker_containers"`
	Endpoints struct {
		Beacon    string `yaml:"beacon"`
		Execution string `yaml:"execution"`
	} `yaml:"endpoints"`
}

type RCloneConfig struct {
	Env             map[string]string `yaml:"env"`
	Version         string            `yaml:"version"`
	Entrypoint      string            `yaml:"entrypoint"`
	CommandTemplate string            `yaml:"cmd_template"`
}

func ReadFromFile(path string) (*Config, error) {
	log.WithField("cfgFile", path).Info("loading config")
	if path == "" {
		path = "config.yaml"
	}
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := &Config{}
	err = yaml.Unmarshal(buf, config)
	if err != nil {
		return nil, err
	}

	log.WithField("count", len(config.Targets.SSH)).Info("ssh targets")
	for _, t := range config.Targets.SSH {

		log.WithFields(log.Fields{
			"alias":  t.Alias,
			"target": fmt.Sprintf("%s@%s:%d", t.User, t.Host, t.Port),
		}).Info("ssh target")
	}

	for k, v := range config.Global.Snapshots.RClone.Env {
		config.Global.Snapshots.RClone.Env[k] = os.ExpandEnv(v)
	}

	return config, nil

}

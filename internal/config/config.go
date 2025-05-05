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
			CheckIntervalSeconds int           `yaml:"check_interval_seconds"`
			BlockInterval        int           `yaml:"block_interval"`
			DryRun               bool          `yaml:"dry_run"`
			RunOnce              bool          `yaml:"run_once"`
			Cleanup              CleanupConfig `yaml:"cleanup"`
			RClone               RCloneConfig  `yaml:"rclone"`
			S3                   S3Config      `yaml:"s3"`
		} `yaml:"snapshots"`
		Database struct {
			Path string `yaml:"path"`
		} `yaml:"database"`
	} `yaml:"global"`
	Server struct {
		ListenAddr string `yaml:"listen_addr"`
		Auth       struct {
			APIToken string `yaml:"api_token"`
		} `yaml:"auth"`
	} `yaml:"server"`
	Targets struct {
		SSH []SSHTargetConfig `yaml:"ssh"`
	} `yaml:"targets"`
}

type CleanupConfig struct {
	Enabled            bool `yaml:"enabled"`
	KeepCount          int  `yaml:"keep_count"`
	CheckIntervalHours int  `yaml:"check_interval_hours"`
}

type S3Config struct {
	BucketName string `yaml:"bucket_name"`
	Region     string `yaml:"region"`
	Endpoint   string `yaml:"endpoint"`
}

type SSHTargetConfig struct {
	Alias            string            `yaml:"alias"`
	Host             string            `yaml:"host"`
	User             string            `yaml:"user"`
	Port             int               `yaml:"port"`
	DataDir          string            `yaml:"data_dir"`
	UploadPrefix     string            `yaml:"upload_prefix"`
	Metadata         map[string]string `yaml:"metadata"`
	DockerContainers struct {
		EngineSnooper string `yaml:"engine_snooper"`
		Execution     string `yaml:"execution"`
		Beacon        string `yaml:"beacon"`
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

// DefaultRCloneCommandTemplate is the default template used for RClone commands if not specified in config
// .DataDir is the directory of the snapshot
// .BucketName is the name of the bucket ( e.g your-bucket-name)
// .UploadPathPrefix is the prefix of the upload path ( e.g mainnet/geth)
// .BlockNumber is the block number of the snapshot (e.g 123456)
const DefaultRCloneCommandTemplate = `-ac "
apk add --no-cache tar zstd jq &&
cd {{ .DataDir }} &&
cat {{ .DataDir }}/_snapshot_metadata.json | jq . &&
tar -I zstd \\
--exclude=./nodekey \\
--exclude=./key \\
--exclude=./discovery-secret \\
-cvf - . \\
| rclone rcat --s3-chunk-size 150M mys3:/{{ .BucketName }}/{{ .UploadPathPrefix }}/{{ .BlockNumber }}/snapshot.tar.zst &&
rclone copy {{ .DataDir }}/_snapshot_eth_getBlockByNumber.json mys3:/{{ .BucketName }}/{{ .UploadPathPrefix }}/{{ .BlockNumber }} &&
rclone copy {{ .DataDir }}/_snapshot_web3_clientVersion.json mys3:/{{ .BucketName }}/{{ .UploadPathPrefix }}/{{ .BlockNumber }} &&
rclone copy {{ .DataDir }}/_snapshot_metadata.json mys3:/{{ .BucketName }}/{{ .UploadPathPrefix }}/{{ .BlockNumber }} &&
echo {{ .BlockNumber }} | rclone rcat mys3:/{{ .BucketName }}/{{ .UploadPathPrefix }}/latest
"`

// GetDefaultRCloneConfig returns an RCloneConfig with sensible defaults
func GetDefaultRCloneConfig() RCloneConfig {
	return RCloneConfig{
		Env:             make(map[string]string),
		Version:         "1.65.2",
		Entrypoint:      "/bin/sh",
		CommandTemplate: DefaultRCloneCommandTemplate,
	}
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

	// Set default values if not specified in config
	if config.Global.Snapshots.RClone.CommandTemplate == "" {
		log.Info("using default RClone command template")
		config.Global.Snapshots.RClone.CommandTemplate = DefaultRCloneCommandTemplate
	}

	if config.Global.Snapshots.RClone.Version == "" {
		config.Global.Snapshots.RClone.Version = "1.65.2"
	}

	if config.Global.Snapshots.RClone.Entrypoint == "" {
		config.Global.Snapshots.RClone.Entrypoint = "/bin/sh"
	}

	// Initialize RClone environment variables from the S3 configuration when available
	if config.Global.Snapshots.S3.Endpoint != "" {
		// Ensure RClone.Env map is initialized
		if config.Global.Snapshots.RClone.Env == nil {
			config.Global.Snapshots.RClone.Env = make(map[string]string)
		}

		// Set the endpoint from S3 config if not explicitly set
		if _, exists := config.Global.Snapshots.RClone.Env["RCLONE_CONFIG_MYS3_ENDPOINT"]; !exists {
			config.Global.Snapshots.RClone.Env["RCLONE_CONFIG_MYS3_ENDPOINT"] = config.Global.Snapshots.S3.Endpoint
		}

		// Set bucket name if available
		if config.Global.Snapshots.S3.BucketName != "" {
			if _, exists := config.Global.Snapshots.RClone.Env["RCLONE_CONFIG_MYS3_BUCKET_NAME"]; !exists {
				config.Global.Snapshots.RClone.Env["RCLONE_CONFIG_MYS3_BUCKET_NAME"] = config.Global.Snapshots.S3.BucketName
			}
		}

		// Set region if available
		if config.Global.Snapshots.S3.Region != "" {
			if _, exists := config.Global.Snapshots.RClone.Env["RCLONE_CONFIG_MYS3_REGION"]; !exists {
				config.Global.Snapshots.RClone.Env["RCLONE_CONFIG_MYS3_REGION"] = config.Global.Snapshots.S3.Region
			}
		}
	}

	log.WithField("count", len(config.Targets.SSH)).Info("ssh targets")
	for _, t := range config.Targets.SSH {
		log.WithFields(log.Fields{
			"alias":  t.Alias,
			"target": fmt.Sprintf("%s@%s:%d", t.User, t.Host, t.Port),
		}).Info("ssh target")
	}

	// Process any environment variables in the configuration
	for k, v := range config.Global.Snapshots.RClone.Env {
		config.Global.Snapshots.RClone.Env[k] = os.ExpandEnv(v)
	}

	// Expand environment variables in S3 configuration
	config.Global.Snapshots.S3.Endpoint = os.ExpandEnv(config.Global.Snapshots.S3.Endpoint)
	config.Global.Snapshots.S3.BucketName = os.ExpandEnv(config.Global.Snapshots.S3.BucketName)
	config.Global.Snapshots.S3.Region = os.ExpandEnv(config.Global.Snapshots.S3.Region)

	// Expand environment variables in SSH configuration
	config.Global.SSH.PrivateKeyPath = os.ExpandEnv(config.Global.SSH.PrivateKeyPath)
	config.Global.SSH.PrivateKeyPassphrasePath = os.ExpandEnv(config.Global.SSH.PrivateKeyPassphrasePath)
	config.Global.SSH.KnownHostsPath = os.ExpandEnv(config.Global.SSH.KnownHostsPath)

	// Expand environment variables in database path
	config.Global.Database.Path = os.ExpandEnv(config.Global.Database.Path)

	// Expand environment variables in SSH target paths
	for i := range config.Targets.SSH {
		config.Targets.SSH[i].DataDir = os.ExpandEnv(config.Targets.SSH[i].DataDir)
	}

	return config, nil
}

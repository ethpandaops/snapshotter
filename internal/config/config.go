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
			CheckIntervalSeconds int             `yaml:"check_interval_seconds"`
			BlockInterval        int             `yaml:"block_interval"`
			DryRun               bool            `yaml:"dry_run"`
			RunOnce              bool            `yaml:"run_once"`
			Cleanup              CleanupConfig   `yaml:"cleanup"`
			RClone               RCloneConfig    `yaml:"rclone"`
			Preimages            PreimagesConfig `yaml:"preimages"`
			S3                   S3Config        `yaml:"s3"`
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
	RootPrefix string `yaml:"root_prefix"`
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
	Preimages PreimagesTargetConfig `yaml:"preimages"`
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
set -o pipefail &&
apk add --no-cache tar zstd jq &&
cd {{ .DataDir }} &&
cat {{ .DataDir }}/_snapshot_metadata.json | jq . &&
tar -I 'zstd -T64' \\
--exclude=./nodekey \\
--exclude=./key \\
--exclude=./discovery-secret \\
--exclude=./_snapshot_preimages \\
-cvf - . \\
| rclone rcat --s3-chunk-size 300M mys3:/{{ .BucketName }}/{{ .UploadPathPrefix }}/{{ .BlockNumber }}/snapshot.tar.zst &&
rclone copy {{ .DataDir }}/_snapshot_eth_getBlockByNumber.json mys3:/{{ .BucketName }}/{{ .UploadPathPrefix }}/{{ .BlockNumber }} &&
rclone copy {{ .DataDir }}/_snapshot_web3_clientVersion.json mys3:/{{ .BucketName }}/{{ .UploadPathPrefix }}/{{ .BlockNumber }} &&
rclone copy {{ .DataDir }}/_snapshot_metadata.json mys3:/{{ .BucketName }}/{{ .UploadPathPrefix }}/{{ .BlockNumber }} &&
echo {{ .BlockNumber }} | rclone rcat mys3:/{{ .BucketName }}/{{ .UploadPathPrefix }}/latest
"`

// PreimagesConfig holds the global command templates for the erigon
// state-preimages export/upload step (see erigontech/erigon#22645).
type PreimagesConfig struct {
	ExportCmdTemplate string `yaml:"export_cmd_template"`
	UploadCmdTemplate string `yaml:"upload_cmd_template"`
}

// PreimagesTargetConfig enables preimage export for a single SSH target.
type PreimagesTargetConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Image    string `yaml:"image"`    // empty = auto-detect from the execution container
	Required bool   `yaml:"required"` // true = preimage failure fails the target/run
}

// PreimagesOutDirName is the scratch directory created inside the target's
// datadir while exporting preimages. Must match the --exclude in
// DefaultRCloneCommandTemplate.
const PreimagesOutDirName = "_snapshot_preimages"

// DefaultPreimagesExportCmdTemplate runs erigon's `snapshots export-preimages`
// as a plain host command (single shell layer, $(...) expands on the host).
// The container user is bound to the datadir owner; a failed stat aborts
// instead of silently running as the image default user.
// Vars: .Image .DataDir .OutDir
const DefaultPreimagesExportCmdTemplate = `u="$(stat -c '%u:%g' {{ .DataDir }})" && docker run --rm --user "$u" -v {{ .DataDir }}:{{ .DataDir }} {{ .Image }} snapshots export-preimages --datadir {{ .DataDir }} --out {{ .OutDir }}`

// DefaultPreimagesUploadCmdTemplate publishes preimages.tar.zst via the rclone
// container. meta is the FIRST tar member (streamable without framed.bin), a
// failed pipe deletes the partial object, and the per-client `latest` stays
// owned by the snapshot template above.
// Editing rule inside -ac "...": write $ as \$ and " as \"; no backticks.
// Vars: .OutDir .BucketName .UploadPathPrefix .BlockNumber
const DefaultPreimagesUploadCmdTemplate = `-ac "
set -o pipefail &&
apk add --no-cache tar zstd &&
cd {{ .OutDir }} &&
tar -I 'zstd -T64' -cf - preimages.meta.json framed.bin \\
| rclone rcat --s3-chunk-size 300M mys3:/{{ .BucketName }}/{{ .UploadPathPrefix }}/{{ .BlockNumber }}/preimages.tar.zst \\
|| ( rclone deletefile mys3:/{{ .BucketName }}/{{ .UploadPathPrefix }}/{{ .BlockNumber }}/preimages.tar.zst; exit 1 )
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
		config.Global.Snapshots.RClone.Version = "1.74.2"
	}

	if config.Global.Snapshots.RClone.Entrypoint == "" {
		config.Global.Snapshots.RClone.Entrypoint = "/bin/sh"
	}

	if config.Global.Snapshots.Preimages.ExportCmdTemplate == "" {
		config.Global.Snapshots.Preimages.ExportCmdTemplate = DefaultPreimagesExportCmdTemplate
	}

	if config.Global.Snapshots.Preimages.UploadCmdTemplate == "" {
		config.Global.Snapshots.Preimages.UploadCmdTemplate = DefaultPreimagesUploadCmdTemplate
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
	config.Global.Snapshots.S3.RootPrefix = os.ExpandEnv(config.Global.Snapshots.S3.RootPrefix)

	// Expand environment variables in SSH configuration
	config.Global.SSH.PrivateKeyPath = os.ExpandEnv(config.Global.SSH.PrivateKeyPath)
	config.Global.SSH.PrivateKeyPassphrasePath = os.ExpandEnv(config.Global.SSH.PrivateKeyPassphrasePath)
	config.Global.SSH.KnownHostsPath = os.ExpandEnv(config.Global.SSH.KnownHostsPath)

	// Expand environment variables in database path
	config.Global.Database.Path = os.ExpandEnv(config.Global.Database.Path)

	// Expand environment variables in SSH target paths
	for i := range config.Targets.SSH {
		config.Targets.SSH[i].DataDir = os.ExpandEnv(config.Targets.SSH[i].DataDir)
		config.Targets.SSH[i].Preimages.Image = os.ExpandEnv(config.Targets.SSH[i].Preimages.Image)
	}

	return config, nil
}

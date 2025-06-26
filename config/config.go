package config

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Config represents the configuration for the application.
type Config struct {
	Cluster    string     `koanf:"cluster"`
	Promtail   Promtail   `koanf:"promtail"`
	Metrics    Metrics    `koanf:"metrics"`
	Scheduling Scheduling `koanf:"scheduling"`
	Budget     Budget     `koanf:"budget"`
	Log        Log        `koanf:"log"`
	Mode       string     `koanf:"mode"`
	KubeConfig string     `koanf:"kube_config"`
	DryRun     bool       `koanf:"dry_run"`
}

type Promtail struct {
	LocalBin string   `koanf:"local_bin"`
	Secret   Secret   `koanf:"secret"`
	Sampling Sampling `koanf:"sampling"`
}

type Sampling struct {
	Selector SamplingSelector `koanf:"selector"`
}

type SamplingSelector struct {
	Format string `koanf:"format"`
}

type Secret struct {
	Name      string `koanf:"name"`
	Namespace string `koanf:"namespace"`
	Key       string `koanf:"key"`
}

type Metrics struct {
	MimirEndpoint string            `koanf:"mimir_endpoint"`
	MimirTenant   string            `koanf:"mimir_tenant"`
	Names         map[string]string `koanf:"names"`
	QueryTimeout  time.Duration     `koanf:"query_timeout"`
}

type Scheduling struct {
	TimeZone string `koanf:"timezone"`
	Cron     Cron   `koanf:"cron"`
}

type Cron struct {
	// IngestionCheck string `koanf:"ingestion_check"`
	BudgetReset string `koanf:"budget_reset"`
}

type Budget struct {
	ConfigPath string  `koanf:"config_path"`
	Org        string  `koanf:"org"`
	Env        string  `koanf:"env"`
	Multiplier float64 `koanf:"multiplier"`
	Minimum    float64 `koanf:"mimimum"`
}

type Log struct {
	Level  string `koanf:"level"`
	Format string `koanf:"format"`
}

type Slack struct {
	WebhookURL string `koanf:"webhook_url"`
	Token      string `koanf:"token"`
	ProxyURL   string `koanf:"proxy_url"`
	Username   string `koanf:"username"`
	Channel    string `koanf:"channel"`
}

const (
	DefaultConfigFile = "/app/config/config.yaml"
)

// Global koanf instance. Use . as the key path delimiter. This can be / or anything.
var (
	k      = koanf.New("config")
	parser = yaml.Parser()
)

func Init() (*Config, error) {
	var configFile string

	if v := os.Getenv("CONFIG_FILE"); v != "" {
		log.Debug().Msg(fmt.Sprintf("env CONFIG_FILE=%s found", v))
		configFile = v
	} else {
		log.Debug().
			Str("default", DefaultConfigFile).
			Msg("CONFIG_FILE env var not found, using default config path")
		configFile = DefaultConfigFile
	}

	cfg, err := New(configFile)
	if err != nil {
		log.Fatal().Err(err).Msg("Error loading config")
	}

	log.Trace().
		Str("config", fmt.Sprintf("%+v", cfg)).
		Msg("Loaded config")

	return &cfg, nil
}

func New(filePath string) (Config, error) {
	log.Debug().Str("filePath", filePath).Msg("Loading Config")
	if err := k.Load(file.Provider(filePath), yaml.Parser()); err != nil {
		log.Fatal().Err(err).Msg("Config parsing error")
		return Config{}, err
	}
	var config Config
	if err := k.Unmarshal("", &config); err != nil {
		log.Fatal().Err(err).Msg("Config unmarshaling error")
		return Config{}, err
	}

	// Set default values for missing fields
	if config.Cluster == "" {
		// panic("cluster name is required")
		log.Panic().Msg("💀 Please provide cluster name!")
	}
	if config.Promtail.LocalBin == "" {
		config.Promtail.LocalBin = "/app/promtail"
		log.Debug().Str("default", config.Promtail.LocalBin).Msg("Promtail binary PATH is not provided, using default")
	}
	if config.Promtail.Secret.Name == "" {
		config.Promtail.Secret.Name = "promtail"
		log.Debug().Str("default", config.Promtail.Secret.Name).Msg("Promtail secret is not provided, using default")
	}
	if config.Promtail.Secret.Namespace == "" {
		config.Promtail.Secret.Namespace = "kube-logging"
		log.Debug().Str("default", config.Promtail.Secret.Namespace).Msg("Promtail secret namespace is not provided, using default")
	}
	if config.Promtail.Secret.Key == "" {
		config.Promtail.Secret.Key = "promtail.yaml"
		log.Debug().Str("default", config.Promtail.Secret.Namespace).Msg("Promtail secret key is not provided, using default")
	}
	if config.Promtail.Sampling.Selector.Format == "" {
		config.Promtail.Sampling.Selector.Format = "{workload=\"%s\"} |= \"\""
		log.Debug().Str("default", config.Promtail.Sampling.Selector.Format).Msg("Promtail sampling selector is not provided, using default")
	}
	if config.Metrics.MimirEndpoint == "" {
		log.Debug().Str("default", config.Metrics.MimirEndpoint).Msg("Mimir endpoint is not provided, using default")
		log.Panic().Msg("💀 Please provide Mimir Endpoint name!")
	}
	if config.Metrics.MimirTenant == "" {
		log.Panic().Msg("💀 Please provide Mimir tenant name!")
	}
	if config.Metrics.QueryTimeout == 0 {
		config.Metrics.QueryTimeout = 30 * time.Second
		log.Debug().Str("default", config.Metrics.QueryTimeout.String()).Msg("Mimir query timeout is not provided, using default")
	}
	if config.Scheduling.TimeZone == "" {
		config.Scheduling.TimeZone = "Asia/Kolkata"
		log.Debug().Str("default", config.Scheduling.TimeZone).Msg("Timezone is not provided, using default")
	}
	if config.Scheduling.Cron.BudgetReset == "" {
		config.Scheduling.Cron.BudgetReset = "0 0 * * *"
		log.Debug().Str("default", config.Scheduling.Cron.BudgetReset).Msg("Reset cron is not provided, using default")
	}
	if config.Budget.ConfigPath == "" {
		config.Budget.ConfigPath = "/app/budget/budget.yaml"
		log.Debug().Str("default", config.Budget.ConfigPath).Msg("Budget config path is not provided, using default")
	}
	if config.Budget.Org == "" {
		log.Panic().Msg("💀 Please provide budget.org name!")
	}
	if config.Budget.Env == "" {
		log.Panic().Msg("💀 Please provide budget.env name!")
	}
	if config.Budget.Multiplier == 0 {
		config.Budget.Multiplier = 1.0
		log.Debug().Float64("default", config.Budget.Multiplier).Msg("Budget baseline multiplier is not provided, using default")
	}
	if config.Budget.Minimum == 0 {
		config.Budget.Minimum = 0.5
		log.Debug().Float64("default", config.Budget.Minimum).Msg("Budget Minimum is not provided, using default")
	}
	if config.Log.Level == "" {
		config.Log.Level = "info"
		log.Debug().Str("default", config.Log.Level).Msg("Log level is not provided, using default")
	}
	if config.Log.Format == "" {
		log.Debug().
			Str("default", config.Log.Level).
			Msg("Log format is not provided, using default")
	}
	if config.Mode == "" {
		config.Mode = "prod"
		log.Debug().Str("default", config.Mode).Msg("Mode is not provided, using default")
	}
	if config.Mode == "prod" {
		config.KubeConfig = ""
		config.Log.Format = "json"
		log.Debug().
			Str("default", config.Log.Format).
			Msg("Using json log format for production mode")
	}
	if config.KubeConfig == "" && config.Mode == "dev" {
		log.Panic().
			Msg("💀 KubeConfig is required in dev mode, will not use use default to prevent accidents 💥")
	}
	if !config.DryRun {
		config.DryRun = false
		log.Debug().
			Bool("default", config.DryRun).
			Msg("DryRun is not provided, using default")
	}

	return config, nil
}

func (c *Config) String() string {

	yamlBytes, err := k.Marshal(parser)

	if err != nil {
		return fmt.Sprintf("failed to marshal config yaml: %v", err)
	}

	return fmt.Sprintf("%+v", string(yamlBytes))
}

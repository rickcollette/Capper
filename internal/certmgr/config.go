package certmgr

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type CertManagerConfig struct {
	Enabled           bool          `yaml:"enabled"`
	DefaultIssuer     string        `yaml:"defaultIssuer"`
	ProductionAllowed bool          `yaml:"productionAllowed"`
	Renewal           RenewalConfig `yaml:"renewal"`
	ACME              ACMEConfig    `yaml:"acme"`
	HTTP01            HTTP01Config  `yaml:"http01"`
	DNS01             DNS01Config   `yaml:"dns01"`
	Storage           StorageConfig `yaml:"storage"`
}

type RenewalConfig struct {
	CheckInterval time.Duration `yaml:"checkInterval"`
	RenewBefore   time.Duration `yaml:"renewBefore"`
	Jitter        time.Duration `yaml:"jitter"`
}

type ACMEConfig struct {
	Accounts []string `yaml:"accounts"`
}

type HTTP01Config struct {
	Enabled       bool   `yaml:"enabled"`
	SolverService string `yaml:"solverService"`
	PathPrefix    string `yaml:"pathPrefix"`
}

type DNS01Config struct {
	Enabled             bool          `yaml:"enabled"`
	DefaultProvider     string        `yaml:"defaultProvider"`
	PropagationTimeout  time.Duration `yaml:"propagationTimeout"`
	CleanupAfterSuccess bool          `yaml:"cleanupAfterSuccess"`
}

type StorageConfig struct {
	KeyEncryption bool   `yaml:"keyEncryption"`
	Backend       string `yaml:"backend"`
}

func DefaultConfig() *CertManagerConfig {
	return &CertManagerConfig{
		Enabled:           true,
		DefaultIssuer:     IssuerLetsEncryptStaging,
		ProductionAllowed: false,
		Renewal: RenewalConfig{
			CheckInterval: 6 * time.Hour,
			RenewBefore:   30 * 24 * time.Hour,
			Jitter:        30 * time.Minute,
		},
		HTTP01: HTTP01Config{
			Enabled:       true,
			SolverService: "internal",
			PathPrefix:    "/.well-known/acme-challenge/",
		},
		DNS01: DNS01Config{
			Enabled:             true,
			DefaultProvider:     "manual",
			PropagationTimeout:  120 * time.Second,
			CleanupAfterSuccess: true,
		},
	}
}

func LoadCertManagerConfig(path string) (*CertManagerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}
	cfg := DefaultConfig()
	return cfg, yaml.Unmarshal(data, cfg)
}

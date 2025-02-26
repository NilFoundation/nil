package connection_manager

import "github.com/jonboulle/clockwork"

type reputationChangeReason string

const (
	ReputationChangeInvalidBlockSignature = reputationChangeReason("invalid block signature")
)

type ReputationChangeSettings = map[reputationChangeReason]Reputation

func DefaultReputationChangeSettings() ReputationChangeSettings {
	return ReputationChangeSettings{
		ReputationChangeInvalidBlockSignature: -100,
	}
}

type Config struct {
	DecayReputationPerSecondPercent uint                     `yaml:"decayReputationPerSecondPercent,omitempty"`
	ReputationBanThreshold          Reputation               `yaml:"reputationBanThreshold,omitempty"`
	ReputationChangeSettings        ReputationChangeSettings `yaml:"reputationChangeSettings,omitempty"`

	clock clockwork.Clock `yaml:"-"`
}

func NewDefaultConfig() *Config {
	return &Config{
		DecayReputationPerSecondPercent: 2, // A bit low, then 35 seconds to reduce reputation by half
		ReputationBanThreshold:          -200,
		ReputationChangeSettings:        DefaultReputationChangeSettings(),
		clock:                           clockwork.NewRealClock(),
	}
}

package connection_manager

import (
	"time"

	"github.com/jonboulle/clockwork"
)

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
	// DecayReputationPerSecondPercent is the percentage of reputation that is lost per second.
	DecayReputationPerSecondPercent uint `yaml:"decayReputationPerSecondPercent,omitempty"`
	// RecalculateReputationsTimeout is the amount of time between recalculating the reputations of all peers.
	RecalculateReputationsTimeout time.Duration `yaml:"recalculateReputationsTimeout,omitempty"`
	// ForgetAfterTime amount of time between the moment we disconnect
	// from a peer and the moment we remove it from the list.
	ForgetAfterTime time.Duration `yaml:"forgetAfterTime,omitempty"`
	// ReputationBanThreshold is the reputation threshold below which a peer is banned.
	ReputationBanThreshold Reputation `yaml:"reputationBanThreshold,omitempty"`
	// ReputationChangeSettings is a map of reasons for reputation changes and the
	// reputation change that should be applied.
	ReputationChangeSettings ReputationChangeSettings `yaml:"reputationChangeSettings,omitempty"`

	clock clockwork.Clock `yaml:"-"`
}

func NewDefaultConfig() *Config {
	return &Config{
		DecayReputationPerSecondPercent: 2, // A bit low, then 35 seconds to reduce reputation by half
		RecalculateReputationsTimeout:   1 * time.Second,
		ForgetAfterTime:                 2 * time.Hour,
		ReputationBanThreshold:          -200,
		ReputationChangeSettings:        DefaultReputationChangeSettings(),
		clock:                           clockwork.NewRealClock(),
	}
}

// Package config reads the settings the system runs on.
//
// Everything here has a default that works, because a joint use engineer
// running one calculation should not have to write a configuration file
// first. What the file is for is the things that differ between the two states
// this utility operates in: the loading district, the grade of construction,
// and which allocation policy applies.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/glinskevxer/downguy/internal/allocate"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/rental"
)

// ErrBadConfig is returned for a configuration that cannot be used.
var ErrBadConfig = errors.New("config: cannot use this configuration")

// Config is the whole set of settings.
type Config struct {
	Database  string     `yaml:"database"`
	Utility   string     `yaml:"utility"`
	District  string     `yaml:"district"`
	Grade     string     `yaml:"grade"`
	Policy    string     `yaml:"policy"`
	Formula   string     `yaml:"formula"`
	Attachers int        `yaml:"attachers"`
	HTTP      HTTPConfig `yaml:"http"`
}

// HTTPConfig is the read side service.
type HTTPConfig struct {
	Address      string `yaml:"address"`
	ReadTimeout  int    `yaml:"read_timeout_seconds"`
	WriteTimeout int    `yaml:"write_timeout_seconds"`
}

// Default returns the settings the system runs on with no file at all.
func Default() Config {
	return Config{
		Database:  "downguy.db",
		Utility:   "SUMMIT VALLEY ELECTRIC",
		District:  string(nesc.Heavy),
		Grade:     string(nesc.GradeC),
		Policy:    allocate.FederalPolicy().Name,
		Formula:   string(rental.FormulaCable),
		Attachers: 5,
		HTTP: HTTPConfig{
			Address:      "127.0.0.1:8471",
			ReadTimeout:  15,
			WriteTimeout: 30,
		},
	}
}

// Load reads a configuration file over the defaults. A missing file is not an
// error: it means the defaults apply, which is the ordinary case.
func Load(path string) (Config, error) {
	cfg := Default()
	if strings.TrimSpace(path) == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("%w: reading %s: %v", ErrBadConfig, path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("%w: parsing %s: %v", ErrBadConfig, path, err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// FromEnv overlays the environment on a configuration. It is here because a
// container deployment sets the database path and the listen address that way
// and nothing else.
func (c Config) FromEnv(lookup func(string) (string, bool)) (Config, error) {
	out := c
	if v, ok := lookup("DOWNGUY_DATABASE"); ok && strings.TrimSpace(v) != "" {
		out.Database = v
	}
	if v, ok := lookup("DOWNGUY_ADDRESS"); ok && strings.TrimSpace(v) != "" {
		out.HTTP.Address = v
	}
	if v, ok := lookup("DOWNGUY_DISTRICT"); ok && strings.TrimSpace(v) != "" {
		out.District = v
	}
	if v, ok := lookup("DOWNGUY_ATTACHERS"); ok && strings.TrimSpace(v) != "" {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return Config{}, fmt.Errorf("%w: DOWNGUY_ATTACHERS is %q", ErrBadConfig, v)
		}
		out.Attachers = n
	}
	if err := out.Validate(); err != nil {
		return Config{}, err
	}
	return out, nil
}

// Validate checks every setting resolves to something the rest of the system
// recognises.
func (c Config) Validate() error {
	if strings.TrimSpace(c.Database) == "" {
		return fmt.Errorf("%w: no database", ErrBadConfig)
	}
	if strings.TrimSpace(c.Utility) == "" {
		return fmt.Errorf("%w: no utility name", ErrBadConfig)
	}
	if _, err := nesc.ParseDistrict(c.District); err != nil {
		return fmt.Errorf("%w: %v", ErrBadConfig, err)
	}
	if _, err := nesc.ParseGrade(c.Grade); err != nil {
		return fmt.Errorf("%w: %v", ErrBadConfig, err)
	}
	if _, err := allocate.PolicyNamed(c.Policy); err != nil {
		return fmt.Errorf("%w: %v", ErrBadConfig, err)
	}
	if _, err := rental.ParseFormula(c.Formula); err != nil {
		return fmt.Errorf("%w: %v", ErrBadConfig, err)
	}
	if c.Attachers < 1 || c.Attachers > 20 {
		return fmt.Errorf("%w: %d attaching entities per pole", ErrBadConfig, c.Attachers)
	}
	if strings.TrimSpace(c.HTTP.Address) == "" {
		return fmt.Errorf("%w: no listen address", ErrBadConfig)
	}
	if c.HTTP.ReadTimeout <= 0 || c.HTTP.WriteTimeout <= 0 {
		return fmt.Errorf("%w: timeouts of %d and %d seconds",
			ErrBadConfig, c.HTTP.ReadTimeout, c.HTTP.WriteTimeout)
	}
	return nil
}

// District returns the loading district as the engineering packages want it.
func (c Config) LoadingDistrict() (nesc.District, error) { return nesc.ParseDistrict(c.District) }

// ConstructionGrade returns the grade of construction.
func (c Config) ConstructionGrade() (nesc.Grade, error) { return nesc.ParseGrade(c.Grade) }

// AllocationPolicy returns the cost allocation policy.
func (c Config) AllocationPolicy() (allocate.Policy, error) { return allocate.PolicyNamed(c.Policy) }

// RateFormula returns the rental formula.
func (c Config) RateFormula() (rental.Formula, error) { return rental.ParseFormula(c.Formula) }

// Marshal writes the configuration back out as yaml, which is how the command
// line prints the settings a run is using.
func (c Config) Marshal() ([]byte, error) {
	out, err := yaml.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("%w: writing the configuration: %v", ErrBadConfig, err)
	}
	return out, nil
}

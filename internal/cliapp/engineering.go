package cliapp

import (
	"io"

	"github.com/glinskevxer/downguy/internal/config"
	"github.com/glinskevxer/downguy/internal/nesc"
)

// Engineering is the loading district and the grade of construction a command
// works in. Four commands were each reading the configuration, then reading a
// district flag over it, then a grade flag over that, in the same twenty lines
// with the same three error paths. This is that, once.
type Engineering struct {
	Config   config.Config
	District nesc.District
	Grade    nesc.Grade
}

// WithEngineering adds the district and grade overrides to a flag set.
func (f *Flags) WithEngineering() *Flags {
	f.set.String("district", "", "the loading district, overriding the configuration")
	f.set.String("grade", "", "the grade of construction, overriding the configuration")
	return f
}

// Engineering resolves the configuration and the two overrides. A flag the
// command did not register is simply absent rather than an error, so a command
// that wants only the district can register only that one.
func (f *Flags) Engineering(env *Env) (Engineering, error) {
	cfg, err := f.Config(env)
	if err != nil {
		return Engineering{}, err
	}
	out := Engineering{Config: cfg}
	if out.District, err = cfg.LoadingDistrict(); err != nil {
		return Engineering{}, err
	}
	if out.Grade, err = cfg.ConstructionGrade(); err != nil {
		return Engineering{}, err
	}
	if override := stringFlag(f, "district"); override != "" {
		if out.District, err = nesc.ParseDistrict(override); err != nil {
			return Engineering{}, err
		}
	}
	if override := stringFlag(f, "grade"); override != "" {
		if out.Grade, err = nesc.ParseGrade(override); err != nil {
			return Engineering{}, err
		}
	}
	return out, nil
}

// stringFlag reads a registered string flag, or the empty string when the
// command never registered it.
func stringFlag(f *Flags, name string) string {
	value := f.Set().Lookup(name)
	if value == nil {
		return ""
	}
	return value.Value.String()
}

// newEngineeringFlags is the flag set every engineering command starts from.
func newEngineeringFlags(name string, out io.Writer) *Flags {
	return NewFlags(name, out).WithJSON().WithEngineering()
}

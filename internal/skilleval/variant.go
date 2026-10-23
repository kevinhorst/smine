package skilleval

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const (
	factorEffort  = "effort"
	factorModel   = "model"
	factorNo      = "no"
	factorOverlay = "overlay"
	factorVersion = "version"

	descriptorNoPrefix = "no-"
	factorSeparator    = ":"
	overlayKeySuffix   = "-overlay"
	versionSeparator   = ".."
)

var ErrBadVariant = errors.New("variant descriptor must change exactly one factor")

var (
	efforts       = []string{"low", "medium", "high", "xhigh", "max"}
	serverPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

type Variant struct {
	Descriptor string
	Factor     string
	Value      string
}

func (v *Variant) apply(base *armConfig) (*armConfig, error) {
	varied := *base
	varied.disabledServers = slices.Clone(base.disabledServers)
	switch v.Factor {
	case factorEffort:
		varied.effort = v.Value
	case factorModel:
		varied.model = v.Value
	case factorNo:
		varied.disabledServers = append(varied.disabledServers, v.Value)
	case factorOverlay:
		varied.variantPatch = v.Value
	case factorVersion:
		return nil, fmt.Errorf("Variant.apply: %s compares stored records, nothing runs: %w", v.Descriptor, ErrBadVariant) //nolint:staticcheck // ST1005: the Receiver.method prefix is the repo's error-message convention
	}

	varied.key = configKey(varied.disabledServers, varied.effort, varied.model)
	if v.Factor == factorOverlay {
		varied.key += overlayKeySuffix
	}
	if varied.key == base.key {
		return nil, fmt.Errorf("Variant.apply: %s equals the baseline %s: %w", v.Descriptor, base.key, ErrBadVariant) //nolint:staticcheck // ST1005: the Receiver.method prefix is the repo's error-message convention
	}
	return &varied, nil
}

func (v *Variant) isValid() bool {
	switch v.Factor {
	case factorEffort:
		return slices.Contains(efforts, v.Value)
	case factorModel:
		return v.Value != "" && !strings.Contains(v.Value, factorSeparator)
	case factorNo:
		return serverPattern.MatchString(v.Value)
	case factorOverlay:
		return filepath.IsLocal(v.Value)
	case factorVersion:
		return isVersionRange(v.Value)
	}
	return false
}

func (v *Variant) isVersion() bool {
	return v.Factor == factorVersion
}

// versions are a version variant's two skill versions, baseline first.
func (v *Variant) versions() (string, string) {
	from, to, _ := strings.Cut(v.Value, versionSeparator)
	return from, to
}

// isVersionRange reports "<a>..<b>" with two distinct non-empty versions.
func isVersionRange(value string) bool {
	from, to, hasRange := strings.Cut(value, versionSeparator)
	isBounded := from != "" && to != ""
	return hasRange && isBounded && from != to
}

func ParseVariant(descriptor string) (*Variant, error) {
	if strings.ContainsAny(descriptor, " \t,+") {
		return nil, fmt.Errorf("ParseVariant: %q changes more than one factor: %w", descriptor, ErrBadVariant)
	}

	factor, value, _ := strings.Cut(descriptor, factorSeparator)
	if server, isNo := strings.CutPrefix(descriptor, descriptorNoPrefix); isNo {
		factor, value = factorNo, server
	}

	variant := &Variant{Descriptor: descriptor, Factor: factor, Value: value}
	if !variant.isValid() {
		return nil, fmt.Errorf(
			"ParseVariant: %q is none of model:<name>, effort:<level>, no-<mcp server>, overlay:<repo-relative patch>, version:<a>..<b>: %w",
			descriptor,
			ErrBadVariant,
		)
	}
	return variant, nil
}

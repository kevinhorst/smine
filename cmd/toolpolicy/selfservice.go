package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/toolpolicy"
	"github.com/kevinhorst/smine/internal/trash"
)

func allowedRules(target string) (*config.Settings, []string, error) {
	settings, err := loadLocalSettings(target)
	if err != nil {
		return nil, nil, fmt.Errorf("allowedRules: %w", err)
	}

	allow, err := permissionList(settings, allowKey)
	if err != nil {
		return nil, nil, fmt.Errorf("allowedRules: %w", err)
	}
	if allow == nil {
		allow = make([]string, 0)
	}
	return settings, allow, nil
}

func appliedVerdict(outcome string) Verdict {
	verdict := Verdict{
		Cause:     causeSettingsApplied,
		Decision:  decisionDeny,
		IsApplied: true,
		Reason: causeSettingsApplied + " — " + unlockSettings + " " + outcome +
			": validated and applied by the gate; this call is refused only so Claude Code does not prompt. Do not retry.",
	}
	return verdict
}

func checkNewRules(rules []string, existing []string, policy *toolpolicy.Policy) ([]string, error) {
	if len(rules) == 0 {
		return nil, errors.New(policy.Causes.SettingsUsage)
	}

	fresh := make([]string, 0, len(rules))
	seen := map[string]bool{}
	for _, rule := range rules {
		if err := isSelfServiceRule(rule, policy.WrapperCommands); err != nil {
			return nil, err
		}
		if seen[rule] {
			return nil, fmt.Errorf("checkNewRules: Rule %q is repeated", rule)
		}

		seen[rule] = true
		if !slices.Contains(existing, rule) {
			fresh = append(fresh, rule)
		}
	}
	return fresh, nil
}

func isSelfServiceRule(raw string, wrapperCommands []string) error {
	rule, err := ParseRule(raw)
	if err != nil {
		return err
	}

	switch rule.Kind {
	case ruleKindBashPrefix:
		fields := strings.Fields(rule.Spec)
		if len(fields) == 0 || strings.ContainsAny(fields[0], "*$`?[") {
			return fmt.Errorf("isSelfServiceRule: Rule %q names no literal program", raw)
		}
		head := filepath.Base(fields[0])
		if slices.Contains(wrapperCommands, head) {
			return fmt.Errorf("isSelfServiceRule: Program %q is a wrapper — the rule would admit every command", head)
		}
		if head == commandHead {
			return fmt.Errorf("isSelfServiceRule: Program %q is the gate itself — whoever runs it supplies the transcript it reads approvals from", head)
		}
		return nil
	case ruleKindDomain:
		if strings.Contains(rule.Spec, "*") {
			return fmt.Errorf("isSelfServiceRule: Domain rule %q carries a wildcard", raw)
		}
		return nil
	case ruleKindMcp:
		isExactTool := strings.Contains(rule.Raw[len(mcpPrefix):], "__") && !strings.Contains(rule.Raw, "*")
		if !isExactTool {
			return fmt.Errorf("isSelfServiceRule: MCP rule %q is not one exact tool", raw)
		}
		return nil
	case ruleKindTool, ruleKindName, ruleKindPathPattern, ruleKindParam:
		return fmt.Errorf("isSelfServiceRule: Rule kind of %q is not self-serviceable", raw)
	}
	return fmt.Errorf("isSelfServiceRule: Rule kind of %q is not self-serviceable", raw)
}

func loadLocalSettings(target string) (*config.Settings, error) {
	settings, err := config.Load(target)
	if err != nil {
		return nil, fmt.Errorf("loadLocalSettings: %w", err)
	}
	return settings, nil
}

// permissionList answers nil for an absent key and an error for a key that is not a string list.
func permissionList(settings *config.Settings, key string) ([]string, error) {
	raw, hasKey := settings.Doc().Get([]string{permissionsKey, key})
	if !hasKey {
		return nil, nil
	}

	list := make([]string, 0)
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("permissionList: permissions.%s is not a string list: %w", key, err)
	}
	return list, nil
}

func saveLocalSettings(target string, settings *config.Settings, keep *trash.Store) error {
	if err := os.MkdirAll(filepath.Dir(target), localSettingsDirMode); err != nil {
		return fmt.Errorf("saveLocalSettings: Failed to create %s: %w", filepath.Dir(target), err)
	}
	if err := config.Save(target, settings, keep); err != nil {
		return fmt.Errorf("saveLocalSettings: %w", err)
	}
	return nil
}

func setPermissionList(settings *config.Settings, key string, values []string) error {
	encoded, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("setPermissionList: %w", err)
	}
	if err := settings.Doc().Set([]string{permissionsKey, key}, encoded); err != nil {
		return fmt.Errorf("setPermissionList: %w", err)
	}
	return nil
}

func uncoveredRules(rules []string, grants *Grants) []string {
	if grants.Approvals[unlockSettings] {
		return nil
	}
	return rules
}

func writePermissionList(target string, settings *config.Settings, key string, values []string, keep *trash.Store) error {
	if err := setPermissionList(settings, key, values); err != nil {
		return fmt.Errorf("writePermissionList: %w", err)
	}
	if err := saveLocalSettings(target, settings, keep); err != nil {
		return fmt.Errorf("writePermissionList: %w", err)
	}
	return nil
}

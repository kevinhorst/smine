package install

import (
	"slices"

	"github.com/kevinhorst/smine/internal/components"
)

// hookBinaries declares Go hook binaries installed to ~/.claude/hooks/bin.
func hookBinaries(names ...string) []binary {
	binaries := make([]binary, 0, len(names))
	for _, name := range names {
		binaries = append(binaries, binary{kind: binaryHook, name: name})
	}
	return binaries
}

// installers returns one installer per catalog entry in catalog order, bound to r where a step depends on the selection.
func installers(r Record) []Component {
	isPeek := slices.Contains(r.Components, components.Peek)
	instructions := file{dirPerm: 0o755, source: "settings/claude_code/AGENTS.md", target: "AGENTS.md"}
	hooksConfig := renderedFile{dirPerm: 0o755, sink: components.SinkHooksConfig, target: hooksConfigTarget}
	heartbeatConfig := renderedFile{dirPerm: privateDirPerm, sink: components.SinkHeartbeatConfig, target: heartbeatConfigTarget}

	all := []Component{
		&artifactComponent{
			binaries: []binary{{kind: binaryPath, name: "rules"}},
			extras:   []extra{baseSettingsExtra(), codexConfigExtra(), serverAgentExtra(isPeek)},
			id:       components.ConfigServer,
			probe:    probeCredential,
		},
		&artifactComponent{extras: []extra{skillSyncExtra(r)}, id: components.Skills},
		&artifactComponent{id: components.SessionHooks, rendered: []renderedFile{hooksConfig}},
		&artifactComponent{
			binaries: hookBinaries("skillenv"),
			id:       components.SkillEnvironment,
		},
		&artifactComponent{
			binaries: hookBinaries("readgate", "globalcontext", "skillcontext", "planmodecontext", "acdslhook"),
			extras:   []extra{alwaysPackExtra()},
			id:       components.Context,
		},
		&artifactComponent{
			binaries: hookBinaries("toolpolicy", "skillinvokegate"),
			extras:   []extra{artifactsDirExtra()},
			id:       components.Gate,
		},
		&artifactComponent{
			binaries: hookBinaries("worktreecreate", "worktreewriteguard"),
			id:       components.Worktrees,
		},
		&artifactComponent{
			binaries: hookBinaries("sessiontitle", "skillhelp", "playsound"),
			id:       components.SessionHelpers,
		},
		&artifactComponent{files: []file{instructions}, id: components.GlobalInstructions},
		&artifactComponent{binaries: []binary{{isRust: true, kind: binaryPath, name: "agentwt"}}, id: components.Agentwt},
		&artifactComponent{extras: []extra{peekBinaryExtra()}, id: components.Peek},
		&artifactComponent{id: components.Ccusage, probe: probeCcusage},
		&artifactComponent{
			binaries: append(hookBinaries("heartbeatfeed"), binary{isRust: true, kind: binaryPath, name: "heartbeat"}),
			extras:   []extra{heartbeatAgentExtra()},
			id:       components.Heartbeat,
			probe:    probeHeartbeat,
			rendered: []renderedFile{heartbeatConfig},
		},
		&artifactComponent{
			binaries: []binary{{kind: binaryPath, name: "routine"}},
			extras:   []extra{routinePlistsExtra(), routineJobsExtra()},
			id:       components.Routines,
		},
		&artifactComponent{id: components.Token, probe: probeToken},
		&artifactComponent{extras: []extra{nightlyJobExtra()}, id: components.Mining, probe: probeNightly},
	}
	return all
}

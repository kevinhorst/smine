package main

import (
	"fmt"
	"io"
	"net"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/skills"
)

// presetOrder and skillModes are the menus' row orders.
var (
	presetOrder = []string{install.PresetMinimal, install.PresetFull, install.PresetCustom}
	skillModes  = []string{skills.SelectionModeAll, skills.SelectionModeChosen, skills.SelectionModeNone}
)

// choice holds -components and -skills values where empty keeps the stored part, plus the closed set prerequisites and build read; isSkillsAsked marks a skill answer whose confirmation travels as -skills-confirmed.
type choice struct {
	componentsArg     string
	ids               []components.ID
	isSkillsAsked     bool
	isSkillsConfirmed bool
	skillsArg         string
}

// skillRow is one line of the skill list: the leaf and the required components the selection lacks.
type skillRow struct {
	missing []components.ID
	name    string
}

// askComponents is the preset menu, then the component list for Custom (concept Flow 1 steps 2-3).
func askComponents(prompter *prompter, stored install.Record, isFound bool, goos string) (string, []components.ID) {
	presets := []string{
		"Minimal — config server and skills; nothing intercepts a session",
		"Full — every component available on " + goos,
		"Custom — pick components",
	}
	fallback := 0
	if isFound {
		fallback = slices.Index(presetOrder, stored.Preset)
	}

	switch prompter.choose("Preset", presets, fallback) {
	case 0:
		return install.PresetMinimal, components.Minimal()
	case 1:
		return install.PresetFull, components.Full(goos)
	}

	selected := components.Minimal()
	if isFound {
		selected = stored.Components
	}
	selected = editComponents(prompter, selected, goos)
	return joinIds(selected), selected
}

// askSkills is the mode menu, the numbered skill list for chosen (an empty list keeps all), then the confirmation that skips the question next time.
func askSkills(prompter *prompter, result *choice, stored install.Record, isFound bool, loadSkills func() ([]skills.Skill, error)) error {
	modes := []string{"all skills the selected components allow", "chosen skills", "none — deployed smine skills are removed"}
	fallback := 0
	if isFound {
		fallback = slices.Index(skillModes, stored.Skills.Mode)
	}

	result.isSkillsAsked = true
	mode := skillModes[prompter.choose("Skills", modes, fallback)]
	result.skillsArg = mode
	if mode == skills.SelectionModeChosen {
		list, err := loadSkills()
		if err != nil {
			return fmt.Errorf("askSkills: %w", err)
		}

		rows := skillRows(list, result.ids)
		selected := editSkills(prompter, knownLeaves(stored.Skills.Chosen, rows), rows)
		result.skillsArg = skills.SelectionModeChosen + ":" + strings.Join(selected, ",")
		if len(selected) == 0 {
			fmt.Fprintln(prompter.out, "no skill chosen — keeping all the selected components allow")
			result.skillsArg = skills.SelectionModeAll
		}
	}

	result.isSkillsConfirmed = prompter.confirm("Keep this skill selection and skip the question on the next installation? (-reset-skills asks again)")
	return nil
}

// choose lets an explicit flag win, then -yes keeps the stored record (Minimal without one, concept D17), then a confirmed skill selection is kept, then the menus ask.
func choose(prompter *prompter, opts *options, stored install.Record, isFound bool, goos, repoDir string) (*choice, error) {
	result := &choice{componentsArg: opts.componentsArg, skillsArg: opts.skillsArg}
	switch {
	case opts.componentsArg != "":
		result.ids = closedIds(opts.componentsArg, goos)
	case opts.isYes && isFound:
		result.ids = stored.Components
	case opts.isYes:
		result.componentsArg = install.PresetMinimal
		result.ids = components.Minimal()
	default:
		result.componentsArg, result.ids = askComponents(prompter, stored, isFound, goos)
	}

	isSkillsSkipped := opts.skillsArg != "" || opts.isYes
	if opts.isResetSkills {
		result.isSkillsAsked = true
		result.isSkillsConfirmed = false
	}
	switch {
	case isSkillsSkipped:
		return result, nil
	case stored.Skills.IsConfirmed && !opts.isResetSkills:
		fmt.Fprintf(prompter.out, "Skills: keeping the confirmed selection (%s) — run with -reset-skills to choose again\n", describeSelection(stored.Skills))
		return result, nil
	}

	loadSkills := func() ([]skills.Skill, error) {
		return repoSkills(repoDir)
	}
	if err := askSkills(prompter, result, stored, isFound, loadSkills); err != nil {
		return nil, fmt.Errorf("choose: %w", err)
	}
	return result, nil
}

// closedIds reads a -components value for prerequisite and build decisions only; configserver stays the validator.
func closedIds(value, goos string) []components.ID {
	switch value {
	case install.PresetMinimal:
		return components.Minimal()
	case install.PresetFull:
		return components.Full(goos)
	}

	ids := []components.ID{components.ConfigServer}
	for name := range strings.SplitSeq(value, ",") {
		ids = append(ids, components.ID(strings.TrimSpace(name)))
	}
	return components.Close(ids)
}

// describeSelection is the stored skill selection as the installer reports it.
func describeSelection(selection install.Selection) string {
	if selection.Mode == skills.SelectionModeChosen {
		return strings.Join(selection.Chosen, ", ")
	}
	return selection.Mode
}

// editComponents toggles rows by number — several per line, space or comma separated — until Enter.
func editComponents(prompter *prompter, selected []components.ID, goos string) []components.ID {
	catalog := components.Catalog()
	for {
		printComponents(prompter.out, catalog, selected, goos)
		answer := prompter.ask("Toggle numbers (3 7 12), Enter when done")
		if answer == "" {
			return selected
		}

		numbers, isValid := parseNumbers(answer, len(catalog))
		if !isValid {
			fmt.Fprintf(prompter.out, "pick 1-%d\n", len(catalog))
			continue
		}
		for _, number := range numbers {
			selected = toggle(prompter, selected, catalog[number-1], goos)
		}
	}
}

// editSkills toggles rows by number — several per line, space or comma separated — until Enter; a row missing components is refused; the result keeps the repo order.
func editSkills(prompter *prompter, selected []string, rows []skillRow) []string {
	for {
		printSkills(prompter.out, rows, selected)
		answer := prompter.ask("Toggle numbers (3 7 12), Enter when done")
		if answer == "" {
			return selected
		}

		numbers, isValid := parseNumbers(answer, len(rows))
		if !isValid {
			fmt.Fprintf(prompter.out, "pick 1-%d\n", len(rows))
			continue
		}
		for _, number := range numbers {
			row := rows[number-1]
			if len(row.missing) > 0 {
				fmt.Fprintf(prompter.out, "%s needs %s — not in this selection\n", row.name, joinIds(row.missing))
				continue
			}
			selected = toggleLeaf(selected, row.name, rows)
		}
	}
}

// installArgs passes the chosen selection plus every flag the caller set (master C6).
func installArgs(opts *options, result *choice) []string {
	args := []string{"-install"}
	if result.componentsArg != "" {
		args = append(args, "-components", result.componentsArg)
	}
	if result.skillsArg != "" {
		args = append(args, "-skills", result.skillsArg)
	}
	if result.isSkillsAsked {
		args = append(args, "-skills-confirmed="+strconv.FormatBool(result.isSkillsConfirmed))
	}
	if opts.presentationProfile != "" {
		args = append(args, "-presentation-profile", opts.presentationProfile)
	}
	if opts.explicit["addr"] {
		args = append(args, "-addr", opts.addr)
	}
	if opts.explicit["peek-port"] {
		args = append(args, "-peek-port", strconv.Itoa(opts.peekPort))
	}
	if opts.explicit["peek-control-port"] {
		args = append(args, "-peek-control-port", strconv.Itoa(opts.peekControlPort))
	}
	if opts.explicit["heartbeat-port"] {
		args = append(args, "-heartbeat-port", strconv.Itoa(opts.heartbeatPort))
	}
	if opts.explicit["init-welcome"] {
		args = append(args, "-init-welcome="+strconv.FormatBool(opts.initWelcome))
	}
	return args
}

// knownLeaves keeps the stored chosen leaves the repo holds and the selection allows, as the list's preselection.
func knownLeaves(chosen []string, rows []skillRow) []string {
	return slices.DeleteFunc(slices.Clone(chosen), func(leaf string) bool {
		index := slices.IndexFunc(rows, func(row skillRow) bool { return row.name == leaf })
		return index < 0 || len(rows[index].missing) > 0
	})
}

func joinIds(ids []components.ID) string {
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		names = append(names, string(id))
	}
	return strings.Join(names, ",")
}

// parseNumbers reads one or more 1-based row numbers, space or comma separated; any token outside 1..count makes the answer invalid.
func parseNumbers(answer string, count int) ([]int, bool) {
	var numbers []int
	for token := range strings.FieldsFuncSeq(answer, func(r rune) bool { return r == ' ' || r == ',' || r == '\t' }) {
		number, err := strconv.Atoi(token)
		isValid := err == nil && number >= 1 && number <= count
		if !isValid {
			return nil, false
		}
		numbers = append(numbers, number)
	}
	return numbers, len(numbers) > 0
}

// printComponents marks with [-] a row this platform cannot install (concept D16).
func printComponents(out io.Writer, catalog []components.Component, selected []components.ID, goos string) {
	for index, component := range catalog {
		mark := "[ ]"
		switch {
		case !components.Available(component.ID, goos):
			mark = "[-]"
		case slices.Contains(selected, component.ID):
			mark = "[x]"
		}
		fmt.Fprintf(out, "%2d %s %s — %s\n", index+1, mark, component.Title, component.Summary)
		if len(component.Requires) > 0 {
			fmt.Fprintf(out, "       needs: %s\n", joinIds(component.Requires))
		}
	}
}

// printSkills lists every repo skill numbered: [x] selected, [-] with its missing components when the selection cannot deploy it.
func printSkills(out io.Writer, rows []skillRow, selected []string) {
	for index, row := range rows {
		mark := "[ ]"
		switch {
		case len(row.missing) > 0:
			mark = "[-]"
		case slices.Contains(selected, row.name):
			mark = "[x]"
		}
		fmt.Fprintf(out, "%2d %s %s", index+1, mark, row.name)
		if len(row.missing) > 0 {
			fmt.Fprintf(out, " — needs %s", joinIds(row.missing))
		}
		fmt.Fprintln(out)
	}
}

func repoSkills(repoDir string) ([]skills.Skill, error) {
	list, err := skills.Scan(filepath.Join(repoDir, "skills"), "")
	if err != nil {
		return nil, fmt.Errorf("repoSkills: %w", err)
	}
	return list, nil
}

// skillRows maps the repo skills to list rows, each naming the required components absent from selected.
func skillRows(list []skills.Skill, selected []components.ID) []skillRow {
	rows := make([]skillRow, 0, len(list))
	for _, skill := range list {
		row := skillRow{name: skill.Name}
		for _, required := range skill.Requires {
			if !slices.Contains(selected, required) {
				row.missing = append(row.missing, required)
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// serverPort is an explicit -addr, else the install record's config-server port.
func serverPort(opts *options, env install.Env) string {
	if !opts.explicit["addr"] {
		return env.Values.ConfigServerPort
	}
	_, port, err := net.SplitHostPort(opts.addr)
	if err != nil {
		return env.Values.ConfigServerPort
	}
	return port
}

// tick adds id and everything it requires (concept D3).
func tick(selected []components.ID, id components.ID) []components.ID {
	return components.Close(append(slices.Clone(selected), id))
}

// toggle refuses the fixed config server and unavailable rows, and unticking asks once about dependents.
func toggle(prompter *prompter, selected []components.ID, component components.Component, goos string) []components.ID {
	switch {
	case component.Fixed:
		fmt.Fprintln(prompter.out, component.Title+" is the fixed base of every installation")
		return selected
	case !components.Available(component.ID, goos):
		fmt.Fprintf(prompter.out, "%s is not available on %s\n", component.Title, goos)
		return selected
	case !slices.Contains(selected, component.ID):
		return tick(selected, component.ID)
	}

	leaving := components.Dependents(component.ID, selected)
	question := fmt.Sprintf("Turning off %s also turns off %s — continue?", component.Title, joinIds(leaving))
	if len(leaving) > 0 && !prompter.confirm(question) {
		return selected
	}
	return untick(selected, component.ID)
}

// toggleLeaf adds or removes leaf, keeping the selection in the rows' order.
func toggleLeaf(selected []string, leaf string, rows []skillRow) []string {
	if slices.Contains(selected, leaf) {
		return slices.DeleteFunc(slices.Clone(selected), func(other string) bool { return other == leaf })
	}

	position := func(name string) int {
		return slices.IndexFunc(rows, func(row skillRow) bool { return row.name == name })
	}
	selected = append(slices.Clone(selected), leaf)
	slices.SortFunc(selected, func(left, right string) int {
		return position(left) - position(right)
	})
	return selected
}

// untick removes id and every selected component that requires it.
func untick(selected []components.ID, id components.ID) []components.ID {
	leaving := append(components.Dependents(id, selected), id)
	return slices.DeleteFunc(slices.Clone(selected), func(other components.ID) bool {
		return slices.Contains(leaving, other)
	})
}

package skillgraph

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Field keys of a format-3 entry, in rendered order.
const (
	FieldKeyScript   = "script"
	FieldKeySchema   = "schema"
	FieldKeyRef      = "ref"
	FieldKeyFlags    = "flags"
	FieldKeyForwards = "forwards"
	FieldKeyReads    = "reads"
	FieldKeyReturns  = "returns"
	FieldKeyMints    = "mints"
	FieldKeyTimeout  = "timeout"
)

// Skill-dir subdirectories the pointer fields sit under.
const (
	schemasDir   = "schemas"
	workflowsDir = "workflows"
)

// fieldBranchPrefix is the frontmatter key a minted branch must start with.
const fieldBranchPrefix = "branch-prefix"

// statementRun opens the statement of a step a timeout belongs to.
const statementRun = "RUN "

var (
	// codeSpanPattern is one backticked identifier a def statement defines.
	codeSpanPattern = regexp.MustCompile("`([A-Za-z_][A-Za-z0-9_]*)`")
	// fieldRoles lists the roles each key may sit on; ref is every role but def and payload.
	fieldRoles = map[string][]string{
		FieldKeyFlags:    {RoleSpawn, RoleStep, RoleSubstep, RoleVerify},
		FieldKeyForwards: {RoleSpawn, RoleStep},
		FieldKeyMints:    {RoleSpawn, RoleStep, RoleStop, RoleSubstep, RoleVerify},
		FieldKeyReads:    {RoleSpawn, RoleStep, RoleSubstep, RoleVerify},
		FieldKeyReturns:  {RoleSpawn, RoleSubstep},
		FieldKeySchema:   {RoleSpawn, RoleStep, RoleSubstep, RoleVerify},
		FieldKeyScript:   {RoleSpawn, RoleSubstep},
		FieldKeyTimeout:  {RoleStep, RoleSubstep},
	}
	// placeholderPattern is one <name …> placeholder of a flags fragment; group 1 must resolve.
	placeholderPattern = regexp.MustCompile(`<([A-Za-z_][A-Za-z0-9_]*)[^>]*>`)
	// templatePattern is one {…} or <…> placeholder a minted value holds.
	templatePattern = regexp.MustCompile(`\{[^}]+\}|<[^>]+>`)
)

// fieldVocabulary is every name a flags placeholder may resolve to, lower-cased.
type fieldVocabulary struct {
	names map[string]bool
}

func newFieldVocabulary(skill *Skill) *fieldVocabulary {
	vocabulary := &fieldVocabulary{names: map[string]bool{}}
	for _, row := range skill.Args() {
		vocabulary.add(row.Name)
	}
	for key := range skill.Env {
		vocabulary.add(key)
	}
	for _, ref := range skill.Entries() {
		entry := ref.Entry
		if entry.Role == RoleDef {
			vocabulary.add(strings.ReplaceAll(entry.Title, " ", "-"))
			for _, match := range codeSpanPattern.FindAllStringSubmatch(entry.Statement, -1) {
				vocabulary.add(match[1])
			}
		}
		if entry.Fields == nil {
			continue
		}

		names := slices.Concat(entry.Fields.Reads, entry.Fields.Returns)
		for _, name := range names {
			vocabulary.add(name)
		}
		if entry.Fields.Mints != nil {
			for name := range entry.Fields.Mints.Id {
				vocabulary.add(name)
			}
		}
	}
	return vocabulary
}

func (v *fieldVocabulary) add(name string) {
	v.names[strings.ToLower(name)] = true
}

func (v *fieldVocabulary) has(name string) bool {
	return v.names[strings.ToLower(name)]
}

// schemaDocument is the slice of a JSON schema the field checks read.
type schemaDocument struct {
	Properties map[string]json.RawMessage `json:"properties"`
}

// channelFieldViolations: a script or schema channel names its file in the field of the same name.
func channelFieldViolations(entry *Entry) []string {
	var violations []string
	hasScript := entry.Fields != nil && entry.Fields.Script != ""
	if entry.Channel == ChannelScript && !hasScript {
		violations = append(violations, entry.Id+": [script] names its stage file in the script field")
	}
	hasSchema := entry.Fields != nil && entry.Fields.Schema != ""
	if entry.Channel == ChannelSchema && !hasSchema {
		violations = append(violations, entry.Id+": [schema] names its schema file in the schema field")
	}
	return violations
}

// checkFieldFiles runs the file-bound field checks: a pointer exists under its dir, a schema holds the names reads, returns and forwards use.
func checkFieldFiles(dir string, skill *Skill) []string {
	if entryFormat(skill) < entryFormatFields {
		return nil
	}

	var violations []string
	for _, ref := range skill.Entries() {
		entry := ref.Entry
		if entry.Fields == nil {
			continue
		}

		fields := entry.Fields
		if fields.Script != "" {
			violations = append(violations, pointerViolations(dir, entry.Id, FieldKeyScript, workflowsDir, fields.Script)...)
		}
		if fields.Schema == "" {
			continue
		}

		violations = append(violations, pointerViolations(dir, entry.Id, FieldKeySchema, schemasDir, fields.Schema)...)
		properties, err := schemaProperties(filepath.Join(dir, fields.Schema))
		if err != nil {
			violations = append(violations, fmt.Sprintf("%s: %s: %v", entry.Id, FieldKeySchema, err))
			continue
		}

		violations = append(violations, nameViolations(entry.Id, FieldKeyReads, fields.Reads, properties)...)
		violations = append(violations, nameViolations(entry.Id, FieldKeyReturns, fields.Returns, properties)...)
		for _, forward := range fields.Forwards {
			if forward.Stage != "" && !properties[forward.Stage] {
				violations = append(violations, fmt.Sprintf("%s: %s: stage %s is no property of %s", entry.Id, FieldKeyForwards, forward.Stage, fields.Schema))
			}
		}
	}
	return violations
}

// checkFields runs the dir-free field checks: the role sets and the value shapes of every key.
func checkFields(skill *Skill) []string {
	if entryFormat(skill) < entryFormatFields {
		return nil
	}

	vocabulary := newFieldVocabulary(skill)
	rows := map[string]bool{}
	for _, row := range skill.Args() {
		rows[row.Name] = true
	}
	branchPrefix := strings.TrimSpace(skill.Field(fieldBranchPrefix))
	var violations []string
	for _, ref := range skill.Entries() {
		entry := ref.Entry
		violations = append(violations, channelFieldViolations(entry)...)
		if entry.Fields == nil {
			continue
		}

		isLeaf := entry.Role == RoleDef || entry.Role == RolePayload
		if isLeaf {
			violations = append(violations, fmt.Sprintf("%s: a %s carries no typed field", entry.Id, entry.Role))
			continue
		}

		fields := entry.Fields
		violations = append(violations, roleViolations(entry, fields)...)
		for _, match := range placeholderPattern.FindAllStringSubmatch(fields.Flags, -1) {
			if !vocabulary.has(match[1]) {
				violations = append(violations, fmt.Sprintf("%s: %s: placeholder <%s> is no Args row, def, read, returned or minted name", entry.Id, FieldKeyFlags, match[1]))
			}
		}
		for _, forward := range fields.Forwards {
			if !rows[forward.Arg] {
				violations = append(violations, fmt.Sprintf("%s: %s: %s is no Args row", entry.Id, FieldKeyForwards, forward.Arg))
			}
			if forward.Stage != "" && fields.Schema == "" {
				violations = append(violations, fmt.Sprintf("%s: %s: stage %s needs a schema on the entry", entry.Id, FieldKeyForwards, forward.Stage))
			}
		}
		hasNames := len(fields.Reads) > 0 || len(fields.Returns) > 0
		if hasNames && fields.Schema == "" {
			violations = append(violations, entry.Id+": reads and returns need a schema on the entry")
		}
		if fields.Mints != nil {
			violations = append(violations, mintsViolations(entry.Id, branchPrefix, fields.Mints)...)
		}
		hasRunTimeout := fields.Timeout > 0 && !strings.Contains(entry.Statement, statementRun)
		if hasRunTimeout {
			violations = append(violations, fmt.Sprintf("%s: %s: only a RUN step takes a timeout", entry.Id, FieldKeyTimeout))
		}
	}
	return violations
}

// checkLabels refuses a labelled item on a format-3 skill: its mechanics are fields.
func checkLabels(skill *Skill) []string {
	if entryFormat(skill) < entryFormatFields {
		return nil
	}

	var violations []string
	for _, ref := range skill.Entries() {
		for _, item := range ref.Entry.Items {
			if item.Label != "" {
				violations = append(violations, fmt.Sprintf("%s: labelled item %s: format 3 carries typed fields, no labelled bullets", ref.Entry.Id, item.Label))
			}
		}
	}
	return violations
}

func mintsViolations(entryId, branchPrefix string, mints *Mints) []string {
	var violations []string
	isEmpty := mints.Branch == "" && mints.File == "" && len(mints.Id) == 0
	if isEmpty {
		violations = append(violations, fmt.Sprintf("%s: %s: names no branch, file or id", entryId, FieldKeyMints))
	}
	if mints.Branch != "" && branchPrefix == "" {
		violations = append(violations, fmt.Sprintf("%s: %s: a minted branch needs the frontmatter branch-prefix", entryId, FieldKeyMints))
	}
	hasForeignBranch := mints.Branch != "" && branchPrefix != "" && !strings.HasPrefix(mints.Branch, branchPrefix)
	if hasForeignBranch {
		violations = append(violations, fmt.Sprintf("%s: %s: branch %s does not start with %s", entryId, FieldKeyMints, mints.Branch, branchPrefix))
	}
	names := slices.Sorted(maps.Keys(mints.Id))
	for _, name := range names {
		if !templatePattern.MatchString(mints.Id[name]) {
			violations = append(violations, fmt.Sprintf("%s: %s: id %s = %q holds no {…} or <…> placeholder", entryId, FieldKeyMints, name, mints.Id[name]))
		}
	}
	return violations
}

func nameViolations(entryId, key string, names []string, properties map[string]bool) []string {
	var violations []string
	for _, name := range names {
		head, _, _ := strings.Cut(name, ".")
		if !properties[head] {
			violations = append(violations, fmt.Sprintf("%s: %s: %s is no property of the entry's schema", entryId, key, name))
		}
	}
	return violations
}

// fieldTexts lists one "<key>: <value>" text per field bullet in rendered key order; renderRef turns a ref id into its text.
func fieldTexts(entry *Entry, renderRef func(id string) string) []string {
	fields := entry.Fields
	if fields == nil {
		return nil
	}

	var texts []string
	add := func(key, value string) {
		texts = append(texts, key+": "+value)
	}
	if fields.Script != "" {
		add(FieldKeyScript, fields.Script)
	}
	if fields.Schema != "" {
		add(FieldKeySchema, fields.Schema)
	}
	for _, id := range fields.Ref {
		add(FieldKeyRef, renderRef(id))
	}
	if fields.Flags != "" {
		add(FieldKeyFlags, fields.Flags)
	}
	for index := range fields.Forwards {
		add(FieldKeyForwards, forwardText(&fields.Forwards[index]))
	}
	if len(fields.Reads) > 0 {
		add(FieldKeyReads, strings.Join(fields.Reads, ", "))
	}
	if len(fields.Returns) > 0 {
		add(FieldKeyReturns, strings.Join(fields.Returns, ", "))
	}
	if fields.Mints != nil {
		for _, text := range mintsTexts(fields.Mints) {
			add(FieldKeyMints, text)
		}
	}
	if fields.Timeout > 0 {
		add(FieldKeyTimeout, strconv.Itoa(fields.Timeout)+" ms")
	}
	return texts
}

// forwardText renders one forwards entry: `<arg>` → `<flag>` [→ stage `<key>`].
func forwardText(forward *Forward) string {
	text := "`" + forward.Arg + "`"
	if forward.Flag != "" {
		text += " → `" + forward.Flag + "`"
	}
	if forward.Stage != "" {
		text += " → stage `" + forward.Stage + "`"
	}
	return text
}

// mintsTexts renders a mints record one line per thing minted, ids sorted by name.
func mintsTexts(mints *Mints) []string {
	var texts []string
	if mints.Branch != "" {
		texts = append(texts, "branch `"+mints.Branch+"`")
	}
	if mints.File != "" {
		texts = append(texts, "file `"+mints.File+"`")
	}
	names := slices.Sorted(maps.Keys(mints.Id))
	for _, name := range names {
		texts = append(texts, "`"+name+"` = `"+mints.Id[name]+"`")
	}
	return texts
}

// pointerViolations: the path sits under its dir and the file exists in the skill dir.
func pointerViolations(dir, entryId, key, under, path string) []string {
	if !strings.HasPrefix(path, under+"/") {
		return []string{fmt.Sprintf("%s: %s: %s is not under %s/", entryId, key, path, under)}
	}
	if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
		return []string{fmt.Sprintf("%s: %s: %s: %v", entryId, key, path, err)}
	}
	return nil
}

func roleViolations(entry *Entry, fields *Fields) []string {
	present := map[string]bool{
		FieldKeyFlags:    fields.Flags != "",
		FieldKeyForwards: len(fields.Forwards) > 0,
		FieldKeyMints:    fields.Mints != nil,
		FieldKeyReads:    len(fields.Reads) > 0,
		FieldKeyReturns:  len(fields.Returns) > 0,
		FieldKeySchema:   fields.Schema != "",
		FieldKeyScript:   fields.Script != "",
		FieldKeyTimeout:  fields.Timeout > 0,
	}
	var violations []string
	keys := slices.Sorted(maps.Keys(present))
	for _, key := range keys {
		if present[key] && !slices.Contains(fieldRoles[key], entry.Role) {
			violations = append(violations, fmt.Sprintf("%s: %s: not on a %s — roles %s", entry.Id, key, entry.Role, strings.Join(fieldRoles[key], ", ")))
		}
	}
	return violations
}

// schemaProperties reads the top-level property names of a JSON schema file.
func schemaProperties(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("schemaProperties: %w", err)
	}

	document := &schemaDocument{}
	if err := json.Unmarshal(data, document); err != nil {
		return nil, fmt.Errorf("schemaProperties: %s: %w", path, err)
	}

	if len(document.Properties) == 0 {
		return nil, fmt.Errorf("schemaProperties: %s declares no properties", path)
	}
	properties := map[string]bool{}
	for name := range document.Properties {
		properties[name] = true
	}
	return properties, nil
}

// FieldTexts lists the entry's field bullets as "<key>: <value>" texts, unwrapped, a ref as its raw id; the diagram's twin of the render.
func FieldTexts(entry *Entry) []string {
	return fieldTexts(entry, func(id string) string { return id })
}

// Package skillgraph holds the typed skill model, the only source of a skill; SKILL.md is rendered from it.
package skillgraph

import (
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/sessiontitle"
)

// Entry roles — the closed format-2 role set.
const (
	RoleDef     = "def"
	RolePayload = "payload"
	RoleQuality = "quality"
	RoleRule    = "rule"
	RoleSpawn   = "spawn"
	RoleStep    = "step"
	RoleStop    = "stop"
	RoleSubstep = "substep"
	RoleVerify  = "verify"
)

// Entry channels a typed field backs: the stage file and the schema file.
const (
	ChannelSchema = "schema"
	ChannelScript = "script"
)

// Section kinds; an empty Kind renders no clause line and means phase.
const (
	SectionKindContext = "context"
	SectionKindPhase   = "phase"
	SectionKindRoute   = "route"
)

// Analysis tiers a skill declares; absent means annotations.
const (
	TierAnnotations = "annotations"
	TierStrict      = "strict"
)

// Runtime placement a skill declares; an absent field means agnostic.
const (
	AnsweredByHook        = "hook"
	DelegationAlways      = "always"
	DelegationNever       = "never"
	DockerAlways          = "always"
	DockerNever           = "never"
	PermissionAcceptEdits = "acceptEdits"
	PermissionBypass      = "bypass"
	PermissionPlanMode    = "planMode"
)

// Frontmatter keys the render derives from the runtime fields; authoring them is refused.
const (
	FieldAnsweredBy         = "answered-by"
	FieldDocker             = "docker"
	FieldEnableDelegation   = "enable-delegation"
	FieldEnforcePermissions = "enforce-permissions"
)

// Role-order ranks: the flow roles share one slot, unknown roles rank last.
const (
	rankDef = iota
	rankRule
	rankQuality
	rankFlow
	rankPayload
	rankUnknown
)

// Atom kinds; an empty Kind means bool.
const (
	KindAtomBool = "bool"
	KindAtomEnum = "enum"
	KindAtomInt  = "int"
)

// Skill dir layout; the help file is read only by the skillhelp hook and fenced off from eval arms.
const (
	DirEval            = "eval"
	DirPayloads        = "payloads"
	DirSmoke           = "smoke"
	FileAnswer         = "answer.md"
	FileCatalog        = "interfaces.cue"
	FileEnforce        = "enforce.json"
	FileEvalDefinition = "definition.json"
	FileHelpTest       = "help-test.md"
	FileManifest       = "SKILL.md"
	FileSmoke          = "smoke.json"
	FileVocabulary     = "vocabulary.cue"
)

// LegacyModelFile is the retired model file name; a dir still holding it is refused.
const LegacyModelFile = "skill.cue"

// modelDecodeLabel names an in-memory decode in its violations.
const modelDecodeLabel = "<leaf>.cue"

const modelExtension = ".cue"

// Sections owning a typed block.
const (
	SectionModel   = "Model"
	SectionRouting = "Routing"
)

// Pipeline edge kinds: from/to and spawnedBy/spawns pair across skills, escalates is one-sided.
const (
	EdgeKindEscalates = "escalates"
	EdgeKindFrom      = "from"
	EdgeKindSpawnedBy = "spawnedBy"
	EdgeKindSpawns    = "spawns"
	EdgeKindTo        = "to"
)

// Bullet labels with a meaning of their own.
const (
	LabelApplies      = "Applies"
	LabelGate         = "Gate"
	LabelHook         = "Hook"
	LabelParam        = "param"
	LabelSchema       = "Schema"
	LabelScript       = "Script"
	LabelSubagentType = "SubagentType"
)

// refPattern is the parser's entryLinkPattern narrowed to the id grammar's segment lengths.
var refPattern = regexp.MustCompile(`\]\(#(skill-[a-z0-9]{2,24}-[a-z0-9]{2,12}-[0-9]{3}[a-z]?)\)`)

// Skill's Vocabulary and Catalog are attached by Load, never written by Save; deploy skips a skill missing one of its Requires.
type Skill struct {
	AnsweredBy         string               `json:"answeredBy,omitempty"`
	Changelog          []ChangelogEntry     `json:"changelog"`
	Docker             string               `json:"docker,omitempty"`
	EnableDelegation   string               `json:"enableDelegation,omitempty"`
	EnforcePermissions []string             `json:"enforcePermissions,omitempty"`
	Env                map[string]*Variable `json:"env,omitempty"`
	Exclusive          [][]string           `json:"exclusive,omitempty"`
	Frontmatter        []Field              `json:"frontmatter"`
	Note               string               `json:"note,omitempty"`
	Preamble           string               `json:"preamble,omitempty"`
	Requires           []components.ID      `json:"requires,omitempty"`
	Sections           []*Section           `json:"sections,omitempty"`
	Tier               string               `json:"tier,omitempty"`

	Atoms      map[string]*Atom          `json:"atoms,omitempty"`
	Catalog    *Catalog                  `json:"-"`
	Dir        string                    `json:"-"`
	Enforce    *Enforce                  `json:"enforce,omitempty"`
	Queries    []Query                   `json:"-"`
	Smoke      *Smoke                    `json:"smoke,omitempty"`
	Title      *sessiontitle.Declaration `json:"title,omitempty"`
	Vocabulary *Vocabulary               `json:"-"`
}

// ChangelogEntry's Tokens is the deployed body's token count for that version, 0 when never counted.
type ChangelogEntry struct {
	Date    string `json:"date"`
	Version string `json:"version"`

	Changes []string `json:"changes,omitempty"`
	Text    string   `json:"text,omitempty"`
	Tokens  int      `json:"tokens,omitempty"`
}

// Field's Value is the text exactly as authored after "key:", quoting, folding and nested lines included.
type Field struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Section is one "## " topic section: its clause line and ordered blocks.
type Section struct {
	Title string `json:"title"`

	Between  string  `json:"between,omitempty"`
	Blocks   []Block `json:"blocks,omitempty"`
	Kind     string  `json:"kind,omitempty"`
	Note     string  `json:"note,omitempty"`
	Pred     string  `json:"pred,omitempty"`
	Replaces string  `json:"replaces,omitempty"`
}

// Block sets exactly one of its fields.
type Block struct {
	Args    *ArgsBlock `json:"args,omitempty"`
	Entry   *Entry     `json:"entry,omitempty"`
	Heading string     `json:"heading,omitempty"`
	Model   *Model     `json:"model,omitempty"`
	Prose   string     `json:"prose,omitempty"`
	Routing *Routing   `json:"routing,omitempty"`
}

// ArgsBlock's bullet fallback serves at most two selector args, Form and Default then empty.
type ArgsBlock struct {
	Bullets bool  `json:"bullets,omitempty"`
	Rows    []Arg `json:"rows"`
}

// Arg's Name carries no backticks; the other cells are verbatim markdown.
type Arg struct {
	Name string `json:"name"`

	Default string `json:"default,omitempty"`
	Effect  string `json:"effect"`
	Form    string `json:"form,omitempty"`
}

// Route is one Disambiguation row — the need and what to use for it; Needs lists the components the target depends on.
type Route struct {
	For string `json:"for"`
	Use string `json:"use"`

	Needs []components.ID `json:"needs,omitempty"`
}

type Routing struct {
	Pipeline      Pipeline `json:"pipeline"`
	Preconditions []string `json:"preconditions"`
	Purpose       string   `json:"purpose"`
	Routes        []Route  `json:"routes"`
}

// Pipeline's Standalone marks a skill that also runs on demand outside the chain.
type Pipeline struct {
	Edges      []Edge `json:"edges,omitempty"`
	Note       string `json:"note,omitempty"`
	Standalone bool   `json:"standalone,omitempty"`
}

// Edge's Optional marks a link the flow may skip; Needs lists the components the linked skill depends on.
type Edge struct {
	Kind  string `json:"kind"`
	Skill string `json:"skill"`

	Needs    []components.ID `json:"needs,omitempty"`
	Optional bool            `json:"optional,omitempty"`
	Route    string          `json:"route,omitempty"`
}

type Model struct {
	Effort string `json:"effort"`
	Reason string `json:"reason"`
	Tier   string `json:"tier"`

	Notes    []string     `json:"notes,omitempty"`
	Routes   []ModelRoute `json:"routes,omitempty"`
	Unviable []string     `json:"unviable,omitempty"`
}

// ModelRoute is one route's tier and effort, overriding the model's own.
type ModelRoute struct {
	Effort string `json:"effort"`
	Route  string `json:"route"`
	Tier   string `json:"tier"`
}

// Entry's ParentSpawn is model-only; a deployed render drops it when a Needs component is absent or an Unless one present.
type Entry struct {
	Id string `json:"id"`

	Channel     string          `json:"channel,omitempty"`
	Custom      []string        `json:"custom,omitempty"`
	Fields      *Fields         `json:"fields,omitempty"`
	Items       []Item          `json:"items,omitempty"`
	Needs       []components.ID `json:"needs,omitempty"`
	Note        string          `json:"note,omitempty"`
	ParentSpawn string          `json:"parentSpawn,omitempty"`
	Payload     *Payload        `json:"payload,omitempty"`
	Post        []string        `json:"post,omitempty"`
	Pred        string          `json:"pred,omitempty"`
	Role        string          `json:"role"`
	Statement   string          `json:"statement,omitempty"`
	Title       string          `json:"title"`
	Unless      []components.ID `json:"unless,omitempty"`
}

// Item is a bullet or a woven fence; Depth 0 is a top-level bullet.
type Item struct {
	Depth  int    `json:"depth,omitempty"`
	Fence  *Fence `json:"fence,omitempty"`
	Label  string `json:"label,omitempty"`
	Marker string `json:"marker,omitempty"`
	Text   string `json:"text,omitempty"`
}

// Fence's Body lines each end in "\n"; "" is a fence with no interior line.
type Fence struct {
	Body   string `json:"body"`
	Info   string `json:"info,omitempty"`
	Marker string `json:"marker"`
}

// Fields are a format-3 entry's typed mechanics; every key has the shape checkFields verifies.
type Fields struct {
	Flags    string    `json:"flags,omitempty"`
	Forwards []Forward `json:"forwards,omitempty"`
	Mints    *Mints    `json:"mints,omitempty"`
	Reads    []string  `json:"reads,omitempty"`
	Ref      []string  `json:"ref,omitempty"`
	Returns  []string  `json:"returns,omitempty"`
	Schema   string    `json:"schema,omitempty"`
	Script   string    `json:"script,omitempty"`
	Timeout  int       `json:"timeout,omitempty"`
}

// Forward says where an invocation arg goes: the flag the command takes it as, the stage-args key it travels under.
type Forward struct {
	Arg   string `json:"arg"`
	Flag  string `json:"flag,omitempty"`
	Stage string `json:"stage,omitempty"`
}

// Mints names what the entry creates: a branch template, a file template, and ids by name with their templates.
type Mints struct {
	Branch string            `json:"branch,omitempty"`
	File   string            `json:"file,omitempty"`
	Id     map[string]string `json:"id,omitempty"`
}

// Payload's body lives in payloads/<entry-id>.md, read by Load and written by Save.
type Payload struct {
	Info   string `json:"info,omitempty"`
	Marker string `json:"marker"`

	Body string `json:"-"`
}

// Atom's Source declares an input atom set by the world instead of granted by a post.
type Atom struct {
	Aliases []string `json:"aliases,omitempty"`
	Kind    string   `json:"kind,omitempty"`
	Members []string `json:"members,omitempty"`
	Sort    string   `json:"sort"`
	Source  string   `json:"source,omitempty"`
}

// Variable is one env declaration: Interface is the bound catalog "<id>@<version>", Required skips the skill when unset.
type Variable struct {
	Interface string `json:"interface"`
	Required  bool   `json:"required,omitempty"`
}

// Catalog is the interface and provider registry in skills/interfaces.cue, attached by Load, never written by Save.
type Catalog struct {
	Interfaces []*Interface `json:"interfaces"`
	Providers  []*Provider  `json:"providers"`
}

// Interface is one capability record; Ref is "<id>@<version>".
type Interface struct {
	Id string `json:"id"`

	Example    string     `json:"example,omitempty"`
	Invocation Invocation `json:"invocation"`
	Kind       string     `json:"kind"`
	Result     Result     `json:"result"`
	Scope      string     `json:"scope"`
	Summary    string     `json:"summary"`
	Validate   Validate   `json:"validate"`
	Version    int        `json:"version"`
}

// Invocation is how a value is called.
type Invocation struct {
	Args    string `json:"args"`
	Cwd     string `json:"cwd"`
	Stdin   string `json:"stdin"`
	Timeout string `json:"timeout"`
}

// Provider is one program with the interface versions it claims; Binary is its install path when Program is not on PATH.
type Provider struct {
	Id string `json:"id"`

	Binary     string        `json:"binary,omitempty"`
	Component  components.ID `json:"component,omitempty"`
	Implements []string      `json:"implements"`
	Probe      []string      `json:"probe"`
	Program    string        `json:"program"`
}

// Result is what an exit code and stdout mean.
type Result struct {
	Exit   map[string]string `json:"exit"`
	Stdout string            `json:"stdout"`
}

// Validate is the deploy-time check per kind: resolves for command and program, exists for path.
type Validate struct {
	Exists   string `json:"exists,omitempty"`
	Resolves bool   `json:"resolves,omitempty"`
}

// Vocabulary is the shared atom registry in skills/vocabulary.cue.
type Vocabulary struct {
	Atoms     map[string]*Atom `json:"atoms"`
	Exclusive [][]string       `json:"exclusive,omitempty"`
}

type EntryRef struct {
	Entry      *Entry
	Section    *Section
	Subsection string
}

// EffectiveKind returns the atom kind with empty meaning bool.
func (a *Atom) EffectiveKind() string {
	if a.Kind == "" {
		return KindAtomBool
	}
	return a.Kind
}

// Bullet ignores nested items, which qualify their parent bullet.
func (e *Entry) Bullet(label string) string {
	for _, item := range e.Items {
		if item.Fence == nil && item.Depth == 0 && item.Label == label {
			return item.Text
		}
	}
	return ""
}

func (e *Entry) EffectiveChannel() string {
	if e.Channel == "" && e.Role != RolePayload {
		return "prose"
	}
	return e.Channel
}

// Params returns the texts of the entry's top-level param items in order.
func (e *Entry) Params() []string {
	var params []string
	for _, item := range e.Items {
		if item.Fence == nil && item.Depth == 0 && item.Label == LabelParam {
			params = append(params, item.Text)
		}
	}
	return params
}

// Refs lists the entry ids the statement, the items and the ref field point at, in order, each once; a short token takes the skill's prefix.
func (e *Entry) Refs(skillName string) []string {
	texts := []string{e.Statement}
	for _, item := range e.Items {
		texts = append(texts, item.Text)
	}
	var ids []string
	for _, text := range texts {
		ids = append(ids, TextRefs(skillName, text)...)
	}
	if e.Fields != nil {
		for _, id := range e.Fields.Ref {
			ids = append(ids, tokenId(skillName, id))
		}
	}
	return uniqueInOrder(ids)
}

// SchemaPointer is the schema field, else the Schema: label a format-2 skill still carries.
func (e *Entry) SchemaPointer() string {
	if e.Fields != nil && e.Fields.Schema != "" {
		return e.Fields.Schema
	}
	return e.Bullet(LabelSchema)
}

// ScriptPointer is the script field, else the Script: label a format-2 skill still carries.
func (e *Entry) ScriptPointer() string {
	if e.Fields != nil && e.Fields.Script != "" {
		return e.Fields.Script
	}
	return e.Bullet(LabelScript)
}

// Anchor keeps a range as "FIRST..LAST" and start/end as written.
func (s *Section) Anchor(skillName string) ([]string, []string, error) {
	replaces, err := parseReplaces(s.Replaces)
	if err != nil {
		return nil, nil, fmt.Errorf("Section.Anchor: %w", err)
	}

	between, err := parseBetween(s.Between)
	if err != nil {
		return nil, nil, fmt.Errorf("Section.Anchor: %w", err)
	}

	prefix := entryIdPrefix + SkillIdName(skillName) + "-"
	return resolveAnchors(replaces, prefix), resolveAnchors(between, prefix), nil
}

// EffectiveKind returns the section kind with empty meaning phase.
func (s *Section) EffectiveKind() string {
	if s.Kind == "" {
		return SectionKindPhase
	}
	return s.Kind
}

// AllAtoms returns the vocabulary atoms overlaid with the skill-local ones.
func (s *Skill) AllAtoms() map[string]*Atom {
	atoms := map[string]*Atom{}
	if s.Vocabulary != nil {
		maps.Copy(atoms, s.Vocabulary.Atoms)
	}
	maps.Copy(atoms, s.Atoms)
	return atoms
}

// Args returns the rows of the skill's Args block, or nil.
func (s *Skill) Args() []Arg {
	for _, section := range s.Sections {
		for _, block := range section.Blocks {
			if block.Args != nil {
				return block.Args.Rows
			}
		}
	}
	return nil
}

// Atom resolves an atom id, skill-local first.
func (s *Skill) Atom(id string) (*Atom, bool) {
	atom, ok := s.AllAtoms()[id]
	return atom, ok
}

// Entries returns every entry in document order with its location.
func (s *Skill) Entries() []EntryRef {
	var refs []EntryRef
	for _, section := range s.Sections {
		subsection := ""
		for _, block := range section.Blocks {
			if block.Heading != "" {
				subsection = block.Heading
			}
			if block.Entry != nil {
				refs = append(refs, EntryRef{Entry: block.Entry, Section: section, Subsection: subsection})
			}
		}
	}
	return refs
}

// Entry finds an entry by id.
func (s *Skill) Entry(id string) (EntryRef, bool) {
	for _, ref := range s.Entries() {
		if ref.Entry.Id == id {
			return ref, true
		}
	}
	return EntryRef{}, false
}

func (s *Skill) Aliases() []string {
	value := s.Field(fieldAliases)
	if value == "" {
		return nil
	}

	aliases := strings.Split(value, ",")
	for index := range aliases {
		aliases[index] = strings.TrimSpace(aliases[index])
	}
	return aliases
}

// Field returns a frontmatter value with surrounding quotes trimmed, or "".
func (s *Skill) Field(key string) string {
	for _, field := range s.Frontmatter {
		if field.Key == key {
			return strings.Trim(strings.TrimSpace(field.Value), `"'`)
		}
	}
	return ""
}

// Model returns the skill's model block, or nil.
func (s *Skill) Model() *Model {
	for _, section := range s.Sections {
		for _, block := range section.Blocks {
			if block.Model != nil {
				return block.Model
			}
		}
	}
	return nil
}

// Routing returns the skill's routing block, or nil.
func (s *Skill) Routing() *Routing {
	for _, section := range s.Sections {
		for _, block := range section.Blocks {
			if block.Routing != nil {
				return block.Routing
			}
		}
	}
	return nil
}

// Name returns the frontmatter name.
func (s *Skill) Name() string {
	return s.Field("name")
}

// Version returns the frontmatter version.
func (s *Skill) Version() string {
	return s.Field("version")
}

// anchorToEntryId keeps a letter suffix lower-case: SKILL-FDESIGN-INTAKE-003b.
func anchorToEntryId(anchor string) string {
	id := strings.ToUpper(anchor)
	last := anchor[len(anchor)-1]
	if last >= 'a' && last <= 'z' {
		id = id[:len(id)-1] + string(last)
	}
	return id
}

// CoverableRole reports whether a role may carry formal clauses.
func CoverableRole(role string) bool {
	switch role {
	case RoleQuality, RoleRule, RoleSpawn, RoleStep, RoleStop, RoleSubstep, RoleVerify:
		return true
	case RoleDef, RolePayload:
		return false
	}
	return false
}

// IsModelFile reports whether path is a skill dir's model file, <dir>/<dir>.cue.
func IsModelFile(path string) bool {
	return filepath.Base(path) == ModelFile(filepath.Base(filepath.Dir(path)))
}

// ModelFile is the model file name of a skill leaf.
func ModelFile(leaf string) string {
	return leaf + modelExtension
}

// ModelPath is the model file of a skill dir, named after the dir.
func ModelPath(dir string) string {
	return filepath.Join(dir, ModelFile(filepath.Base(dir)))
}

func RoleOrderRank(role string) int {
	switch role {
	case RoleDef:
		return rankDef
	case RoleRule:
		return rankRule
	case RoleQuality:
		return rankQuality
	case RoleSpawn, RoleStep, RoleStop, RoleSubstep, RoleVerify:
		return rankFlow
	case RolePayload:
		return rankPayload
	}
	return rankUnknown
}

// Schema of the machine-canonical skill model (internal/skillgraph).
// A skill's model file <leaf>.cue unifies with #Skill; skills/vocabulary.cue with #Vocabulary; skills/interfaces.cue with #Catalog.
// Structural rules live here. Cross-references, format-dependent caps and
// display-length caps live in check.go. Conditional rules key only on
// fields that are always concrete — required (role) or defaulted (kind) —
// because an absent optional field cannot be referenced.

#EntryId:     =~#"^SKILL-[A-Z0-9]{2,24}-[A-Z0-9]{2,12}-[0-9]{3}[a-z]?$"#
#EntryRef:    =~#"^(SKILL-[A-Z0-9]{2,24}-)?[A-Z0-9]{2,12}-[0-9]{3}[a-z]?$"#
#AtomId:      =~#"^[A-Za-z_][A-Za-z0-9_]*$"#
#CustomTag:   =~#"^[a-z][a-z0-9-]*:[a-z0-9:-]*[a-z0-9-]$"#
#FenceMarker: =~#"^`{3,}$"#
#Line:        =~#"^[^\n]*$"#
#Name:        =~#"^[A-Za-z_][A-Za-z0-9_.]*$"#
#RelPath:     =~#"^[^/~\n][^\n]*$"# & !~#"(^|/)\.\.(/|$)"#
#Title:       =~#"^[^*\s]+(\s+[^*\s]+){0,2}$"#

#ComponentId: "config-server" | "skills" | "session-hooks" | "skill-environment" | "context" | "gate" | "worktrees" | "session-helpers" | "global-instructions" | "agentwt" | "peek" | "ccusage" | "heartbeat" | "routines" | "token" | "mining"

#EnvName: =~#"^[A-Z][A-Z0-9_]*$"#

#InterfaceId:  =~#"^[a-z][a-z0-9-]*$"#
#InterfaceRef: =~#"^[a-z][a-z0-9-]*@[0-9]+$"#
#Kind:         "command" | "program" | "path" | "skill"
#Scope:        "install" | "repo"

#Catalog: {
	interfaces!: [...#Interface]
	providers!:  [...#Provider]
}

// #Interface is one capability a skill may bind; a changed contract is a new version, never an edit.
#Interface: {
	id!:      #InterfaceId
	version!: int & >=1
	kind!:    #Kind
	scope!:   #Scope
	summary!: #Line & != ""
	example?: #Line & != ""
	invocation!: {
		cwd!:     #Line & != ""
		args!:    #Line & != ""
		stdin!:   #Line & != ""
		timeout!: #Line & != ""
	}
	result!: {
		exit!: {[string]: #Line & != ""}
		stdout!: #Line & != ""
	}
	validate!: {
		resolves?: bool
		exists?:   "dir" | "file"
	}
}

// #Provider names the program a component installs and the interface versions it implements.
#Provider: {
	id!:         #InterfaceId
	program!:    string
	binary?:     #Line & != ""
	component?:  #ComponentId
	implements!: [...#InterfaceRef]
	probe!:      ["interfaces"]
}

#PermissionMode: "acceptEdits" | "planMode" | "bypass"

#Role:    "def" | "payload" | "quality" | "rule" | "spawn" | "step" | "stop" | "substep" | "verify"
#Channel: "gate" | "hook" | "prose" | "schema" | "script"

#Skill: {
	answeredBy?:         "hook"
	changelog!:          [#ChangelogEntry, ...#ChangelogEntry]
	docker?:             "always" | "never"
	enableDelegation?:   "always" | "never"
	enforce?:            #Enforce
	enforcePermissions?: [...#PermissionMode]
	env?: {[#EnvName]: #Variable}
	exclusive?:          [...[...#AtomId]]
	frontmatter!:        [#Field, ...#Field]
	note?:               string
	preamble?:           string
	requires?:           [...#ComponentId]
	sections?:           [...#Section]
	smoke?:              #Smoke
	tier?:               "annotations" | "strict"
	title?:              #TitleBlock
	atoms?: {[#AtomId]: #Atom}
}

// #Variable binds one catalog interface; the key is the interface id uppercased, the rendered name SKILL_<SKILL>_<KEY>.
#Variable: {
	interface!: #InterfaceRef
	required?:  bool
}

#ChangelogEntry: {
	date!:    =~#"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"#
	version!: =~#"^[0-9]+(\.[0-9]+){1,2}$"#
	tokens?:  int & >0
	{text!: string & != ""} | {changes!: [string & != "", ...string & != ""]}
}

#Field: {
	key!:   =~#"^[a-z][a-z0-9-]*$"#
	value!: string
}

#Section: {
	title!:    #Line & != ""
	between?:  #Line & != ""
	blocks?:   [...#Block]
	kind:      *"" | "context" | "phase" | "route"
	note?:     string
	pred?:     #Line & != ""
	replaces?: #Line & != ""
	if kind == "route" {
		pred!: string
	}
	if kind != "route" {
		between?:  error("between: only a route section carries an anchor clause")
		pred?:     error("pred: only a route section carries a pred")
		replaces?: error("replaces: only a route section carries an anchor clause")
	}
}

#Block: {prose!: string & != ""} | {heading!: #Line & != ""} | {entry!: #Entry} | {args!: #ArgsBlock} | {routing!: #Routing} | {model!: #Model}

#Entry: {
	id!:          #EntryId
	channel?:     #Channel
	custom?:      [...#CustomTag]
	fields?:      #Fields
	items?:       [...#Item]
	needs?:       [...#ComponentId]
	note?:        string
	parentSpawn?: #EntryId
	payload?:     #Payload
	post?:        [...#AtomId]
	pred?:        #Line & != ""
	role!:        #Role
	statement?:   #Line
	title!:       #Title
	unless?:      [...#ComponentId]
	if role == "def" || role == "payload" {
		fields?: error("fields: a def or payload carries no typed field")
		pred?:   error("pred: formal clauses belong to coverable roles (rule, quality, spawn, step, stop, substep, verify)")
	}
	if role == "def" || role == "payload" || role == "quality" || role == "rule" {
		post?: error("post: only flow roles (spawn, step, stop, substep, verify) grant atoms")
	}
	if role == "payload" {
		channel?: error("channel: a payload entry takes no channel tag")
		payload!: #Payload
	}
	if role != "payload" {
		payload?: error("payload: only a payload entry carries a payload fence")
	}
}

#Item: {
	depth?:  int & >=0
	label?:  #Line & != "" & !~#"^(pred|post)$"#
	marker!: "*" | "-"
	text!:   #Line & != ""
} | {
	fence!: #Fence
}

#Fence: {
	body!:   string
	info?:   #Line
	marker!: #FenceMarker
}

// #Fields is a format-3 entry's typed mechanics; role sets, placeholder resolution and file existence live in fields.go.
#Fields: {
	flags?:    #Line & != ""
	forwards?: [#Forward, ...#Forward]
	mints?:    #Mints
	reads?:    [#Name, ...#Name]
	ref?:      [#EntryRef, ...#EntryRef]
	returns?:  [#Name, ...#Name]
	schema?:   #RelPath
	script?:   #RelPath
	timeout?:  int & >0
}

#Forward: {
	arg!:   #Line & != ""
	flag?:  =~#"^-"#
	stage?: #Name
}

#Mints: {
	branch?: #Line & != ""
	file?:   #RelPath
	id?: {[#Name]: #Line & != ""}
}

#Payload: {
	body?:   error("body: a payload body lives in payloads/<entry-id>.md")
	info?:   #Line
	marker!: #FenceMarker
}

#ArgsBlock: {
	bullets?: bool
	rows!:    [#Arg, ...#Arg]
}

#Arg: {
	name!:    #Line & != "" & !~"`"
	default?: #Line
	effect!:  #Line
	form?:    #Line
}

#Route: {
	for!:   #Line
	use!:   #Line
	needs?: [...#ComponentId]
}

#SkillName: =~#"^[a-z][a-z0-9-]*$"#
#Tier:      "frontier" | "mid-tier" | "small"
#Effort:    "high" | "low" | "max" | "medium" | "xhigh"

#Routing: {
	pipeline!:      #Pipeline
	preconditions!: [#Line & != "", ...(#Line & != "")]
	purpose!:       #Line & != ""
	routes!:        [#Route, ...#Route]
}

#Pipeline: {
	edges?:      [...#Edge]
	note?:       #Line & != ""
	standalone?: bool
}

#Edge: {
	kind!:     "escalates" | "from" | "spawnedBy" | "spawns" | "to"
	skill!:    #SkillName
	needs?:    [...#ComponentId]
	optional?: bool
	route?:    #Line & != ""
}

#Model: {
	effort!:   #Effort
	reason!:   #Line & != ""
	tier!:     #Tier
	notes?:    [...(#Line & != "")]
	routes?:   [#ModelRoute, ...#ModelRoute]
	unviable?: [...(#Line & != "")]
}

#ModelRoute: {
	effort!: #Effort
	route!:  #Line & != ""
	tier!:   #Tier
}

// One atom shape for skill-local and shared atoms.
#Atom: {
	aliases?: [...string]
	kind:     *"" | "bool" | "enum" | "int"
	members?: [...#AtomId]
	sort!:    #Line & != ""
	source?:  #Line & != ""
	if kind == "enum" {
		members!: [#AtomId, ...#AtomId]
	}
	if kind != "enum" {
		members?: error("members: only an enum atom declares members")
	}
}

#InputMatch: {
	field!:   string & != ""
	pattern!: string & != ""
}

// #Enforce is a skill's runtime enforcement: stateless gate rules, heartbeat's
// anchors and checks over them, and the judgment points of the runtime agent.
#Enforce: {
	anchors?: {[#EnforceId]: #Anchor}
	checks?: {[#EnforceId]: #Check}
	gate?: #GateRules
	judgment?: {[#EnforceId]: #Judgment}
}

#EnforceId: =~"^[a-z][a-z0-9_]*$"

#GateRules: {
	planModeDenied?: [...string]
	rootDenied?: [...string]
	workflowOnly?: [...string]
	writers?: [...{
		agentTypes!: [string, ...string]
		glob!:       string & != ""
	}]
}

#Anchor: {
	agent?: "root" | "any" | "subagent"
	capture?: [...{
		name!:    #EnforceId
		pointer!: string
	}]
	exclude?: [...#InputMatch]
	input?: [...#InputMatch]
	tool!: string & != ""
}

// A check kind accepts exactly its own anchor fields.
#Check: {
	kind!:     "provenance" | "requires" | "phase" | "stop"
	reason!:   string & != ""
	recovery!: string & != ""
	if kind == "provenance" {
		arg!:     string & != ""
		capture!: #EnforceId
		from!:    #EnforceId
		on!:      #EnforceId
	}
	if kind == "requires" {
		after!: #EnforceId
		on!:    #EnforceId
		since?: #EnforceId
	}
	if kind == "phase" {
		deny!:  #EnforceId
		from!:  #EnforceId
		until!: #EnforceId
	}
	if kind == "stop" {
		after!:   #EnforceId
		require!: #EnforceId
	}
}

#Judgment: {
	anchor!: #EnforceId
	evidence!: [string, ...string]
	question!: string & != ""
	send!:     string & != ""
}

// #TitleBlock is a skill's session-title declaration: the states its approval
// gates put into the title, each bound to the gate entry that runs
// `smine-rules title set <TOKEN>`. Token cap, entry resolution and the
// set-command check live in check.go.
#TitleBlock: {
	states!: {[#TitleToken]: #TitleState}
}

#TitleToken: =~"^[A-Z]+(-[A-Z]+)*$"

#TitleState: {
	entry!:   #EntryId
	meaning!: #Line & != ""
}

// #Smoke is a skill's smoke declaration: a skip with its reason, or one run
// on a dummy input with its expectation. Anchor resolution, the interactive
// rule, the env keys and the fixture paths live in check.go and Load.
#Smoke: #SmokeSkip | #SmokeRun

#SmokeSkip: {
	skip!: #Line & != ""
}

#SmokeRun: {
	effort?:  #Effort
	env?: {[#EnvName]: #Line & != ""}
	expect!:  #SmokeAnchors | *#SmokeCheck
	fixture?: #SmokeFixture
	invoke!:  #Line
	limits?:  #SmokeLimits
	mode?:    "interactive" | "nightly"
	model?:   string
}

#SmokeAnchors: {
	anchors!:    [#EnforceId, ...#EnforceId]
	minAnchors?: number & >=0 & <=1
	minChecks?:  number & >=0 & <=1
}

#SmokeCheck: {
	check?: [string, ...string]
}

#SmokeFixture: {
	patch?: string & != ""
	tree?:  string & != ""
}

#SmokeLimits: {
	minutes?: int & >0
	usd?:     number & >0
}

#Vocabulary: {
	atoms!: {[#AtomId]: #Atom}
	exclusive?: [...[...#AtomId]]
}

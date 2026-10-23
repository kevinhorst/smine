package skillgraph

// provideCompleteSkill is a format-2 skill that passes Check and renders:
// routing and args blocks, a phase section with rule, steps and a payload,
// and a route section inserted after FLOW-002.
func provideCompleteSkill() *Skill {
	return &Skill{
		Changelog: []ChangelogEntry{{Date: "2000-01-01", Text: "fixture", Version: "1.0"}},
		Frontmatter: []Field{
			{Key: "name", Value: " demo"},
			{Key: "description", Value: " demo fixture. Trigger on nothing."},
			{Key: "version", Value: " 1.0"},
			{Key: "format", Value: " 2"},
		},
		Preamble: "# Demo\n\nOne-paragraph purpose.",
		Sections: []*Section{
			{
				Title: "Routing",
				Blocks: []Block{
					{Routing: &Routing{
						Pipeline: Pipeline{
							Edges:      []Edge{{Kind: EdgeKindTo, Skill: "fdesign"}, {Kind: EdgeKindEscalates, Optional: true, Route: "change", Skill: "fimplement"}},
							Note:       "demo note",
							Standalone: true,
						},
						Preconditions: []string{"a demo request"},
						Purpose:       "demo.",
						Routes:        []Route{{For: "planning", Use: "/fdesign"}},
					}},
				},
			},
			{
				Title: "Args",
				Blocks: []Block{
					{Args: &ArgsBlock{Rows: []Arg{{Name: "mode", Default: "`a`", Effect: "picks a mode", Form: "`a\\|b`"}}}},
				},
			},
			{
				Title: "Flow",
				Blocks: []Block{
					{Entry: &Entry{Id: "SKILL-DEMO-FLOW-001", Pred: "not panic", Role: RoleRule, Statement: "Agent MUST stay calm.", Title: "Stay calm"}},
					{Entry: &Entry{
						Id:        "SKILL-DEMO-FLOW-002",
						Items:     []Item{{Label: LabelParam, Marker: "*", Text: "one fact."}},
						Post:      []string{"first_done"},
						Role:      RoleStep,
						Statement: "RUN the first step.",
						Title:     "First step",
					}},
					{Entry: &Entry{Id: "SKILL-DEMO-FLOW-003", Pred: "first_done", Role: RoleStep, Statement: "RUN the second step.", Title: "Second step"}},
					{Entry: &Entry{
						Id:        "SKILL-DEMO-TPL-001",
						Payload:   &Payload{Body: "## TLDR\n", Info: "markdown", Marker: "```"},
						Role:      RolePayload,
						Statement: "The template.",
						Title:     "Template",
					}},
				},
			},
			{
				Title:   "Nightly",
				Between: "FLOW-002..end",
				Blocks: []Block{
					{Entry: &Entry{Id: "SKILL-DEMO-NIGHT-001", Role: RoleStep, Statement: "RUN at night.", Title: "Night step"}},
				},
				Kind: SectionKindRoute,
				Pred: "mode = nightly",
			},
			{
				Title: "Model",
				Blocks: []Block{
					{Model: &Model{
						Effort: "high",
						Notes:  []string{"A demo note."},
						Reason: "demo reasoning",
						Routes: []ModelRoute{{Effort: "low", Route: "plan", Tier: "small"}},
						Tier:   "frontier",
					}},
				},
			},
		},
		Atoms: map[string]*Atom{
			"first_done": {Sort: "state"},
			"panic":      {Sort: "state"},
		},
	}
}

// flowSection returns the fixture's Flow section.
func flowSection(skill *Skill) *Section {
	return skill.Sections[2]
}

// routeSection returns the fixture's Nightly route section.
func routeSection(skill *Skill) *Section {
	return skill.Sections[3]
}

// provideVersionedSkill is the fixture with one changelog row per version, newest first, the frontmatter on the top row.
func provideVersionedSkill(versions ...string) *Skill {
	skill := provideCompleteSkill()
	skill.Changelog = make([]ChangelogEntry, 0, len(versions))
	for _, version := range versions {
		skill.Changelog = append(skill.Changelog, ChangelogEntry{Date: "2000-01-01", Text: "fixture " + version, Version: version})
	}
	setVersion(skill, versions[0])
	return skill
}

// fixtureMarkdown is the canonical rendering of provideCompleteSkill.
const fixtureMarkdown = "---\n" +
	"name: demo\n" +
	"description: demo fixture. Trigger on nothing.\n" +
	"version: 1.0\n" +
	"format: 2\n" +
	"---\n" +
	"\n" +
	"# Demo\n" +
	"\n" +
	"One-paragraph purpose.\n" +
	"\n" +
	"## Routing\n" +
	"\n" +
	"**Purpose:** demo.\n" +
	"\n" +
	"**Disambiguation:**\n" +
	"\n" +
	"| For | Use |\n" +
	"| :--- | :--- |\n" +
	"| planning | /fdesign |\n" +
	"\n" +
	"**Preconditions:**\n" +
	"- a demo request\n" +
	"\n" +
	"**Pipeline:** standalone; **demo** → /fdesign; escalates to /fimplement change *(optional)* — demo note\n" +
	"\n" +
	"## Args\n" +
	"\n" +
	"| Arg | Form | Default | Effect |\n" +
	"| :-- | :--- | :------ | :----- |\n" +
	"| `mode` | `a\\|b` | `a` | picks a mode |\n" +
	"\n" +
	"## Flow\n" +
	"\n" +
	"`{pred: not panic}`\\\n" +
	"**SKILL-DEMO-FLOW-001** `[rule]` **Stay calm**\n" +
	"\n" +
	"Agent MUST stay calm.\n" +
	"\n" +
	"`{post: first_done}`\\\n" +
	"**SKILL-DEMO-FLOW-002** `[step]` **First step**\n" +
	"\n" +
	"RUN the first step.\n" +
	"* param: one fact.\n" +
	"\n" +
	"`{pred: first_done}`\\\n" +
	"**SKILL-DEMO-FLOW-003** `[step]` **Second step**\n" +
	"\n" +
	"RUN the second step.\n" +
	"\n" +
	"**SKILL-DEMO-TPL-001** `[payload]` **Template**\n" +
	"\n" +
	"The template.\n" +
	"\n" +
	"```markdown\n" +
	"## TLDR\n" +
	"```\n" +
	"\n" +
	"## Nightly\n" +
	"`{section: route, pred: mode = nightly, between: FLOW-002..end}`\n" +
	"\n" +
	"**SKILL-DEMO-NIGHT-001** `[step]` **Night step**\n" +
	"\n" +
	"RUN at night.\n" +
	"\n" +
	"## Model\n" +
	"\n" +
	"- Suggested: frontier / high; plan route: small / low\n" +
	"- Reason: demo reasoning\n" +
	"- Tested unviable: — (none yet)\n" +
	"- A demo note.\n"

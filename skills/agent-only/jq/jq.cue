changelog: [{
	date:    "2026-10-04"
	version: "1.12"
	text:    "References reworded to name no smine program."
	tokens:  573
}, {
	date:    "2026-10-04"
	version: "1.11"
	text:    "Smoke reads acdsl/registry.json; proposals/schema.json was retired"
	tokens:  589
}, {
	date:    "2026-09-28"
	version: "1.10"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  589
}, {
	date:    "2026-08-23"
	version: "1.9"
	text:    "Retargeted the skill-map pointer to README.md § Skill map (docs reorg)"
}, {
	date:    "2026-08-11"
	version: "1.8"
	text:    "utility-user reference updated smine-summary → smine-batch (smine-summary retired)"
}, {
	date:    "2026-07-30"
	version: "1.7"
	text:    "allowed-tools permission manifest declared"
}, {
	date:    "2026-07-30"
	version: "1.6"
	text:    "moved under skills/agent/ group; name and behavior unchanged"
}, {
	date:    "2026-07-27"
	version: "1.5"
	text:    "reference renames — ssummarize → smine-summary, couchskill-eval → skillroutine-eval"
}, {
	date:    "2026-07-24"
	version: "1.4"
	text:    "delegation declaration removed — small skills are never delegatable"
}, {
	date:    "2026-07-22"
	version: "1.3"
	text:    "classification unattended-safe (Delegation + Command surface), effort low"
}, {
	date:    "2026-07-19"
	version: "1.2"
	text:    "reference rename: eval-skill → couchskill-eval; moved under skills/util/"
}, {
	date:    "2026-07-13"
	version: "1.1"
	text:    "When-to-use section standardized to match other skills"
}, {
	date:    "2026-06-29"
	version: "1.0"
	text:    "initial version"
}]
frontmatter: [{
	key:   "name"
	value: " jq"
}, {
	key:   "description"
	value: " Extract specific fields from JSON files efficiently using jq instead of reading entire files, saving 80-95% context."
}, {
	key: "metadata"
	value: """

		  mcpmarket-version: 1.0.0
		"""
}, {
	key:   "author"
	value: " tychohq"
}, {
	key:   "version"
	value: " 1.12"
}, {
	key:   "allowed-tools"
	value: " Read, Bash(jq *)"
}]
preamble: """
	# jq: JSON Data Extraction Tool

	Use jq to extract specific fields from JSON files without loading entire file contents into context.
	"""
sections: [{
	title: "When to use"
	blocks: [{
		prose: """
			**Use when:** extracting specific fields from JSON/JSONL files — large files where only a subset is needed, nested structure
			queries, filtering/transforming structured data. Saves 80-95% context vs reading entire files.
			**Don't use when:** file is small (<50 lines) or you need to understand overall structure — just use Read. Making edits that
			need full context — Read first. The data is not JSON/JSONL.
			**Workflow position:** standalone — used as a utility by other skills (see README.md § Skill
			map, smine repo).
			"""
	}]
}, {
	title: "Common File Types"
	blocks: [{
		prose: """
			JSON files where jq excels:
			- package.json, tsconfig.json
			- Lock files (package-lock.json, yarn.lock in JSON format)
			- API responses
			- Configuration files
			"""
	}]
}, {
	title: "Quick Examples"
	blocks: [{
		prose: """
			```bash
			# Get version from package.json
			jq -r .version package.json

			# Get nested dependency version
			jq -r '.dependencies.react' package.json

			# List all dependencies
			jq -r '.dependencies | keys[]' package.json
			```
			"""
	}]
}, {
	title: "Core Principle"
	blocks: [{
		prose: "Extract exactly what is needed in one command - massive context savings compared to reading entire files."
	}]
}, {
	title: "Detailed Reference"
	blocks: [{
		prose: """
			For comprehensive jq patterns, syntax, and examples, load [jq guide](./reference/jq-guide.md):
			- Core patterns (80% of use cases)
			- Real-world workflows
			- Advanced patterns
			- Pipe composition
			- Error handling
			- Integration with other tools
			"""
	}]
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "low"
			reason: "recipe lookup + jq invocation"
			tier:   "small"
		}
	}]
}]
smoke: {
	expect: {}
	invoke: "print the top-level keys of acdsl/registry.json"
}

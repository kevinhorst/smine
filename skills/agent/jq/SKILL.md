---
name: jq
description: Extract specific fields from JSON files efficiently using jq instead of reading entire files, saving 80-95% context.
metadata:
  mcpmarket-version: 1.0.0
author: tychohq
version: 1.7
allowed-tools: Bash(jq *), Read
---
# jq: JSON Data Extraction Tool

Use jq to extract specific fields from JSON files without loading entire file contents into context.

## When to use

**Use when:** extracting specific fields from JSON/JSONL files — large files where only a subset is needed, nested structure queries, filtering/transforming structured data. Saves 80-95% context vs reading entire files.
**Don't use when:** file is small (<50 lines) or you need to understand overall structure — just use Read. Making edits that need full context — Read first. The data is not JSON/JSONL.
**Workflow position:** standalone — used as a utility by other skills (smine-summary, skillroutine-eval) (see `docs/skill-map.md`, smine repo).

## Common File Types

JSON files where jq excels:
- package.json, tsconfig.json
- Lock files (package-lock.json, yarn.lock in JSON format)
- API responses
- Configuration files

## Quick Examples

```bash
# Get version from package.json
jq -r .version package.json

# Get nested dependency version
jq -r '.dependencies.react' package.json

# List all dependencies
jq -r '.dependencies | keys[]' package.json
```

## Core Principle

Extract exactly what is needed in one command - massive context savings compared to reading entire files.

## Detailed Reference

For comprehensive jq patterns, syntax, and examples, load [jq guide](./reference/jq-guide.md):
- Core patterns (80% of use cases)
- Real-world workflows
- Advanced patterns
- Pipe composition
- Error handling
- Integration with other tools

## Model

- Suggested: small / low
- Reason: recipe lookup + jq invocation
- Tested unviable: — (none yet)

## Changelog

- v1.7 (2026-07-30): allowed-tools permission manifest declared
- v1.6 (2026-07-30): moved under skills/agent/ group; name and behavior unchanged
- v1.5 (2026-07-27): reference renames — ssummarize → smine-summary, couchskill-eval → skillroutine-eval
- v1.4 (2026-07-24): delegation declaration removed — small skills are never delegatable
- v1.3 (2026-07-22): classification unattended-safe (Delegation + Command surface), effort low
- v1.2 (2026-07-19): reference rename: eval-skill → couchskill-eval; moved under skills/util/
- v1.1 (2026-07-13): When-to-use section standardized to match other skills
- v1.0 (2026-06-29): initial version

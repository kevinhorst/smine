---
name: jq
description: Extract specific fields from JSON files efficiently using jq instead of reading entire files, saving 80-95% context.
metadata:
  mcpmarket-version: 1.0.0
author: tychohq
version: 1.9
allowed-tools: Bash(jq *), Read
---
# jq: JSON Data Extraction Tool

Use jq to extract specific fields from JSON files without loading entire file contents into context.

## When to use

**Use when:** extracting specific fields from JSON/JSONL files — large files where only a subset is needed, nested structure queries, filtering/transforming structured data. Saves 80-95% context vs reading entire files.
**Don't use when:** file is small (<50 lines) or you need to understand overall structure — just use Read. Making edits that need full context — Read first. The data is not JSON/JSONL.
**Workflow position:** standalone — used as a utility by other skills (smine-batch, skillroutine-eval) (see README.md § Skill map, smine repo).

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

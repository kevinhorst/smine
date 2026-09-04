<!-- synced from smine — do not edit; repo-owned files in this dir are overlays (see README.md) -->

# SQL CODE STYLE RULES (MINIMAL)

**Files:** `*.sql`

## BASELINE

SQL MUST be:

* readable
* consistent
* structured for analytical queries (CTEs preferred)

---

## KEYWORD FORMATTING

**RULE-SQL-001** `[review]` — SQL keywords are UPPERCASE.

* SQL keywords MUST be UPPERCASE

Examples:

```sql
SELECT
FROM
WHERE
GROUP BY
ORDER BY
JOIN
```

---

## SELECT FORMAT

**RULE-SQL-002** `[review]` — SELECT clauses are multiline — one column per line.

SELECT clauses MUST be multiline:

```sql
SELECT
  column_a,
  column_b,
  function_call(...) AS alias
```

* One column per line
* Trailing comma REQUIRED (except last line)

---

## ALIASING

**RULE-SQL-003** `[review]` — All computed fields carry explicit aliases.

* All computed fields MUST have explicit aliases
* Table aliases MUST be short and consistent

Examples:

```sql
raw_reports r
tag_reports tr
visibility_durations vd
```

* Aliases SHOULD be 1–3 characters

---

## CTE USAGE

**RULE-SQL-004** `[review]` — Complex queries use CTEs (`WITH`).

* Complex queries MUST use CTEs (`WITH`)

* Each logical step MUST be its own CTE

* CTE names MUST describe their purpose

Examples:

```sql
counters
raw_reports
tag_reports
visibility_durations
```

---

## CTE STRUCTURE

**RULE-SQL-005** `[review]` — Each CTE follows the canonical structure shown below.

Each CTE MUST follow this structure:

```sql
cte_name AS (
  SELECT
    ...
  FROM
    ...
  WHERE
    ...
  GROUP BY
    ...
)
```

* Keywords MUST be on new lines
* Indentation MUST be consistent

---

## JOINS

**RULE-SQL-006** `[review]` — JOINs are written explicitly with their conditions.

JOINs MUST be written explicitly:

```sql
FROM
  table_a a
INNER JOIN
  table_b b
ON
  a.id = b.id
```

* NEVER use implicit joins
* JOIN type MUST always be specified

---

## WHERE CONDITIONS

**RULE-SQL-007** `[review]` — Conditions are vertically aligned.

Conditions MUST be vertically aligned:

```sql
WHERE
  condition_a
  AND condition_b
  AND condition_c
```

* Each condition on its own line

---

## FUNCTIONS

**RULE-SQL-008** `[review]` — Complex nested functions go multiline.

* Nested functions MUST be multiline if complex

Example:

```sql
LEAST(
  100.0,
  GREATEST(
    0.0,
    ...
  )
)
```

---

## NULL HANDLING

**RULE-SQL-009** `[review]` — NULL handling is explicit.

* NULL handling MUST be explicit

Preferred:

```sql
IF(value IS NULL, 0, value)
SAFE_DIVIDE(...)
SAFE_MULTIPLY(...)
```

---

## NAMING

**RULE-SQL-010** `[review]` — Columns and aliases use PascalCase or camelCase consistently.

* Columns and aliases MUST use PascalCase or camelCase consistently
* Avoid snake_case unless required by schema

---

## ORDERING

**RULE-SQL-011** `[review]` — ORDER BY is explicit.

ORDER BY MUST be explicit:

```sql
ORDER BY
  column_a,
  column_b
```

* No positional ordering (`ORDER BY 1`)

---

## COMMENTS

**RULE-SQL-012** `[review]` — Complex logic is explained with short comments.

* Complex logic SHOULD be explained with short comments
* CTEs MAY include a short description above

---

## ENFORCEMENT

* Queries MUST be readable without mental parsing
* Prefer splitting logic into CTEs instead of nesting
* Do NOT compress queries into single lines

---

## DATABASE MCP TOOLS

### WHEN THIS APPLIES

Any task touching database schema, tables, columns, or query behavior:
writing or reviewing queries, designing migrations, debugging data issues,
grounding claims about what a table actually contains.

### BASELINE

**RULE-SQL-DBMCP-001** `[review]` — When the JetBrains IDE MCP server is available, its database tools are the preferred way to run queries.

* If the JetBrains IDE MCP server (`goland`/`jetbrains`) is available, its
  database tools are the source of truth for live schema state
* Do NOT reconstruct schema from migration files, ORM models, or memory when
  the MCP can answer directly — those describe intent, the IDE connection
  shows what is actually deployed

Tools, by use case:

* `list_database_connections` — which databases the IDE knows about
* `list_database_schemas` / `list_schema_objects` — what exists in a schema
* `get_database_object_description` — exact columns, types, keys of a table
* `preview_table_data` — what the data actually looks like
* `execute_sql_query` — verify query behavior against real data

### FALLBACK

**RULE-SQL-DBMCP-002** `[review]` — The MCP server only exists while the IDE is open with the project loaded — fall back to the CLI client otherwise.

* The MCP server only exists while the IDE is open with the project loaded
* If its tools are absent or the connection test fails, fall back to
  migrations/models and say so — do not present schema derived from files as
  verified live state

### ENFORCEMENT

* Schema claims in plans, reviews, and diagnoses MUST state their source:
  IDE MCP (verified) or files (unverified)
* Read-only tools may be used freely; `execute_sql_query` with writes follows
  the same confirmation rules as any other state-changing action

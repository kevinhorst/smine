# stylecheck

Guards the learned style profile that /smine-style writes and every session receives: any secret or PII match replaces the profile with the withheld stub.

## Usage

```
stylecheck [--path <file>]
```

- `--path` — the style profile to check; defaults to the presentation package's style profile path.

## How it works

**Output**
- One `<path>: <kind> line <n>` per hit — never the matched text — then `<path>: withheld — replaced by the stub`; `<path>: ok` when clean.

**Exit codes**

| Code | Meaning |
|---|---|
| 0 | clean, or no profile |
| 1 | hit: profile withheld and replaced by the stub |
| 2 | error: profile unscanned or stub unwritten |

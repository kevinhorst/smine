# evalstats

Computes per-side descriptive statistics and Welch's t-test comparisons of the variant against the baseline from an A/B dir's `ab.json` (written by `skilleval ab-collect`), and writes `stats.json` next to it. Called by the /skillroutine-ab flow after collection.

## Usage

```
usage: evalstats -ab <ab.json> -out <stats.json>
```

## How it works

**Groups**
- Each side groups its records' measures by measure name, in discovery order: the baseline side first, measures sorted within a record.
- Each group gets n, mean, sample (n-1) variance and standard deviation; an empty group is noted `no usable values`.

**Comparisons**
- For every measure, each non-baseline side is compared with the baseline side: delta of means, Welch's t, degrees of freedom, and significance.
- Significance is two-sided at alpha 0.05 from a t-critical table; no p-values are computed.
- Fewer than two replicas per side gives df 0 and the note `insufficient replicas for significance (need >=2 per arm)`.
- A baseline side with no values at all is an error.

**Exit codes**

| Code | Meaning |
|---|---|
| 0 | `stats.json` written |
| 2 | missing flag, unreadable or unparsable `ab.json`, no baseline values, or write failure |

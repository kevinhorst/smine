# Clamp — smoke spec

The change adds package `internal/smokereview` with one function and its test.

- `Clamp(value, low, high int) int` returns `low` when `value < low`, `high` when `value > high`, and `value` otherwise.
- The bounds are inclusive: `Clamp(low, low, high)` is `low` and `Clamp(high, low, high)` is `high`.
- Reversed bounds are swapped before clamping: `Clamp(3, 5, 1)` is `3` and `Clamp(9, 5, 1)` is `5`.
- `TestClamp` covers a value below the range, one above it, one inside it and reversed bounds, and asserts the exact result of each case.

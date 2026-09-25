# AGENTS.md

Guidelines for AI agents and contributors.

## Documentation

- Spell out acronyms on first use (e.g., "Application Programming Interface (API)"), then the acronym alone.

## CI / Automation

- Run everything through `Taskfile.yml`; shell one-liners in docs/scripts won't reproduce in CI.

## Commit Messages

- Never add agent names as author or co-author. Commits reflect the human contributor only.

## Bug Fixes

- Reproduce first, add a failing test case before the fix, and never merge a bug fix without a regression test.

## Technical Decisions

- Weight correctness, readability, simplicity, and long-term maintainability over development cost and time. Choose what we'd live with for years.

## Observability

- Prefer structured logging (key/value, consistent levels, machine-parseable) over unstructured strings.
- For servers, also expose Prometheus metrics (counters, gauges, histograms) on a standard scrape endpoint.

## Go

- Before hand-rolling, search stdlib and direct deps: `go doc <pkg>`, `go doc <pkg>.<Symbol>`, `go doc -all <pkg>`. Reuse the existing symbol over a private reimplementation.
- Near-fit, not exact fit: wrap with a small adapter; do not fork a parallel implementation.

## Maintenance

- Keep this file current with key decisions and workflows. Update it in the same change that a decision or workflow changes.

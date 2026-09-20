# AGENTS

## Documentation

- Decisions should be recorded in [MADR 4.0.0](https://adr.github.io/madr/) format in `docs/adrs`.
- Plans should be stored in `docs/agents/plans`.
- Keep `docs/agents/internals.md` up to date as repository changes.
- Let's use full-length lines when writing docs.

## Coding standards

- Versions should always be pinned.

### Testing

#### Philosophy

- Write tests that verify the *observable behavior* of this function from a caller's perspective. Each test should answer: "if this test fails, what promise to callers has been broken?"
- When designing tests:
  - First, list the test cases you plan on covering (happy path, edge cases, error handling).
  - Then write the tests.
  - Flag any behavior in the code that seems untestable or buggy rather than writing tests that enshrine it.
- Tracing and observability should be first-class citizens.

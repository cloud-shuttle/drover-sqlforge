## Summary

Brief description of what this PR does and why.

Closes #

## Type of change

- [ ] Bug fix
- [ ] New feature
- [ ] Refactor (no behaviour change)
- [ ] Documentation
- [ ] New warehouse plugin

## Checklist

- [ ] `go test ./...` passes locally
- [ ] `make e2e` passes locally
- [ ] New code has unit tests (aim for the changed package to have >70% coverage)
- [ ] If a new HTTP endpoint or WASM boundary was added: fuzz test added (`go test -fuzz=FuzzName -fuzztime=10s`)
- [ ] If this changes the `virtual.Runner` interface: all built-in runners updated
- [ ] If this changes plan/apply behaviour: E2E test updated in `test/e2e/`
- [ ] If this is a significant architectural decision: ADR added in `docs/adr/`
- [ ] Documentation updated (tutorial, how-to, or reference as appropriate)

## Testing notes

Describe how you tested this. Include any non-obvious edge cases you checked.

## Dialect impact

Does this change affect all warehouses equally, or is it dialect-specific? Which runners were tested?

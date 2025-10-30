# AGENTS
1. Use Go 1.25.3 toolchain; module mode is default.
2. Run builds via `make build` (sets GOEXPERIMENT=jsonv2).
3. Clean binaries with `make clean`; binaries land in `bin/`.
4. Run all tests with `make test` or `go test ./...`.
5. Run a single test with `go test ./path/to/pkg -run '^TestName$'`.
6. Integration tests require `OPENAI_API_KEY`; optionally set Zotero envs.
7. Keep code gofmt-clean; run `go fmt ./...` before committing.
8. Use goimports-style import blocks: stdlib, blank line, third-party, local.
9. Prefer explicit contexts: `ctx context.Context` as the first parameter.
10. Return errors instead of panicking; wrap with `fmt.Errorf("...: %w", err)`.
11. Log only in entrypoints (e.g. server startup); libraries should propagate errors.
12. Use descriptive PascalCase for exported types/functions, lowerCamelCase for locals.
13. Keep structs in `models/` as the shared data contracts across layers.
14. When working with goroutines, guard shared data and respect cancellation.
15. Database changes run inside transactions with deferred rollback until commit.
16. Resource URIs should follow existing `pdf://{documentId}` patterns.
17. Tests should use `t.Run`, `t.Skip`, and avoid network calls without env gates.
18. Comments on exported symbols explain behavior and invariants.
19. Update MCP tool schemas via `jsonschema.For` and keep handlers returning typed responses.
20. Prefer dependency injection for storage/LLM clients; avoid globals.

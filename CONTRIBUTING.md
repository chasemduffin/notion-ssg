# Contributing

Contributions from humans and agents are welcome.

Keep changes small and reviewable. Include tests for behavior changes, avoid committing secrets or generated local output, and document any prompt/context used for agentic contributions when it materially affects the diff.

Before submitting:

```sh
gofmt -w .
go test ./...
```

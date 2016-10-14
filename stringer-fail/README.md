# Stringer does not rebuild when needed

1. `go get -d github.com/tschottdorf/goplay/stringer-fail`
1. `go generate ./...`

  This returns:
  ```
  stringer: checking package: main.go:6:2: could not import github.com/tschottdorf/goplay/stringer-fail/subpkg (can't find import: github.com/tschottdorf/goplay/stringer-fail/subpkg)
  main.go:9: running "stringer": exit status 1
  ```
1. `go build -i ./...` (need the `-i` flag!)
1. `go generate ./...` works.
1. `go clean -i ./...` to reset to non-working.

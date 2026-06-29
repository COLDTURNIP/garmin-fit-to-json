# garmin-fit-to-json

Convert Garmin FIT activity files to a curated JSON summary with one native executable.

## Usage

```sh
fit2json input.fit output.json
```

## Install

```sh
go install github.com/COLDTURNIP/garmin-fit-to-json/cmd/fit2json@latest
```

Ensure `$GOBIN` or `$HOME/go/bin` is on your `PATH`.

## Build

```sh
make build
```

Direct build command:

```sh
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=dev" -o dist/fit2json ./cmd/fit2json
```

## Test

```sh
make test
```

`make validate` runs formatting checks, `go vet`, unit tests, and the binary build.

## Output schema

Top-level JSON sections:

- `metadata`
- `summary`
- `timeline.laps` with integer lap-level `start_heart_rate` and `end_heart_rate` fields. Missing lap heart-rate values are emitted as `0`.
- `timeline.events`
- `series.record`

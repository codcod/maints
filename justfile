binary  := "maints"
version := `git describe --tags --always --dirty 2>/dev/null || echo "dev"`

build:
    go build -ldflags="-X main.version={{version}}" -o {{binary}} ./cmd/maints

install:
    go install -ldflags="-X main.version={{version}}" ./cmd/maints

test:
    go test -v -race -count=1 ./...

clean:
    rm -f {{binary}}
    rm -rf triaged-maints/

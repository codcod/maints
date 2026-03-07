binary := "triage"

build:
    go build -o {{binary}} .

install:
    go install .

test:
    go test -v -race -count=1 ./...

clean:
    rm -f {{binary}}
    rm -rf triaged-maints/

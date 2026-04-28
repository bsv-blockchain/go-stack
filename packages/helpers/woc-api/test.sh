#!/bin/sh
echo "TESTING"
cd $(dirname "$0")

PROG_NAME=$(awk -F'"' '/^const progname =/ {print $2}' main.go)
PROG_NAME_UPPER=$(echo $PROG_NAME | awk '{ print toupper($0) }')

echo "Running tests for $PROG_NAME"

#go vet .
go get -u github.com/vakenbolt/go-test-report/
go test ./test/ -json | go-test-report
exit 1




#!/bin/bash

set -eu

./build.sh

go test -v ./...
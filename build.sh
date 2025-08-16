#!/bin/bash

set -eu

mkdir -p .bin

go build -v -o .bin/nostall main.go


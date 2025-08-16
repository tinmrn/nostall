#!/bin/bash

set -eu

./build.sh

exec .bin/nostall "$@"


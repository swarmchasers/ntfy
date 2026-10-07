#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
umask 077
mkdir -p observer/private-data
exec ./dist/ntfy-observer serve --config observer/server.yml

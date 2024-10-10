#!/bin/bash

function logError {
	echo "Error: $1"
	exit 1
}

# Quick checks
command -v go >/dev/null || logError "go command not found."
command -v tar >/dev/null || logError "tar command not found."
command -v base64 >/dev/null || logError "base64 command not found."

# Build go binary - Google's gopacket pcap cannot be statically compiled
# Archs: amd64,arm64
export GOARCH="amd64"
export GOOS=linux
go build -o wol-server-pve-$GOARCH-dynamic -a -tags purego -ldflags '-s -w' wakeonlanpve.go || logError "failed to compile binary"

exit 0

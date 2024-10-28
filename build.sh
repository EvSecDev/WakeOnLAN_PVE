#!/bin/bash
set -e

# Quick checks
command -v go >/dev/null
command -v tar >/dev/null
command -v base64 >/dev/null

# Build go binary - Google's gopacket pcap cannot be statically compiled
# Archs: amd64,arm64
export GOARCH="amd64"
export GOOS=linux
go build -o wol-server-pve-$GOARCH-dynamic -a -tags purego -ldflags '-s -w' wakeonlanpve.go

exit 0

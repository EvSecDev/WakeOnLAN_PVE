#!/bin/bash

function musl_build_static() {
    local arch os repoRoot localOutputFile srcDir localOutputPath
    arch=$1
    os=$2
    repoRoot=$3
    localOutputFile=$4

    if ! [[ $arch =~ amd64 ]]
    then
        echo "Refusing to build musl binary for archecture $arch (only amd64 is allowed)" >&2
        exit 1
    fi
    if ! [[ $os =~ linux ]]
    then
        echo "Refusing to build musl binary for os $os (only linux is allowed)" >&2
        exit 1
    fi

    localOutputPath="$repoRoot/$localOutputFile"

    if [[ -x docker ]]
    then
        mkdir /tmp/build
        cp "$repoRoot/.build_helpers/alpine_builder.sh.txt" /tmp/build/alpine_builder.sh
        cp -r ./ /tmp/build
        cd /tmp/build
        docker run --rm --pull always -v "$PWD":/mnt alpine /mnt/alpine_builder.sh
        mv /tmp/build/out "$localOutputPath"
        rm -rf /tmp/build
    else
        ssh DebTest 'mkdir /tmp/build'
        scp "$repoRoot/.build_helpers/alpine_builder.sh.txt" DebTest:/tmp/build/alpine_builder.sh
        scp -r ./ DebTest:/tmp/build
        ssh DebTest 'cd /tmp/build && docker run --rm --pull always -v "$PWD":/mnt alpine /mnt/alpine_builder.sh'
        scp DebTest:/tmp/build/out "$localOutputPath"
        ssh DebTest 'rm -rf /tmp/build'
    fi

    if ! [[ -f $localOutputPath ]]
    then
        echo "Failed build - no output received" >&2
        exit 1
    fi
}
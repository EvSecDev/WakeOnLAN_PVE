#!/bin/bash

function prepare_github_release_files() {
	local programNamePrefix tempDir
	programNamePrefix=$1

	if [[ -d $HOME/Downloads ]]
	then
		tempDir="$HOME/Downloads/releasetemp"
	elif [[ -d $HOME/.local/tmp ]]
	then
		tempDir="$HOME/.local/tmp/releasetemp"
	elif [[ -d /tmp ]]
	then
		tempDir="/tmp/releasetemp"
	else
		echo -e "${RED}ERROR${RESET}: Unable to identify available temp dir, cannot continue" >&2
		exit 1
	fi

	mkdir -p "$tempDir"
	if [[ $? != 0 ]]
	then
		echo -e "${RED}ERROR${RESET}: Unable to create temp release dir $tempDir, cannot continue" >&2
		exit 1
	fi

	mv "$programNamePrefix"* "$tempDir"/
	if [[ $? != 0 ]]
	then
		echo -e "${RED}ERROR${RESET}: Unable to move binaries in temp release dir, cannot continue" >&2
		exit 1
	fi

	echo "$tempDir"
}
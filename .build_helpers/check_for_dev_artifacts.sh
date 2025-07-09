#!/bin/bash
command -v git >/dev/null

function check_for_dev_artifacts() {
	local src repoDir headCommitHash lastReleaseCommitHash lastReleaseVersionNumber currentVersionNumber
	src=$1
	repoDir=$2

	echo "[*] Checking for development artifacts in source code..."

	# Get head commit hash
	headCommitHash=$(git rev-parse HEAD)

    # Get commit where last release was generated from
    lastReleaseCommitHash=$(cat "$repoDir"/.last_release_commit)

	# Retrieve the program version from the last release commit
	lastReleaseVersionNumber=$(git show "$lastReleaseCommitHash":"$src"/main.go 2>/dev/null | grep "progVersion string" | cut -d" " -f5 | sed 's/"//g')

	# Get the current version number
	currentVersionNumber=$(grep "progVersion string" "$src"/main.go | cut -d" " -f5 | sed 's/"//g')

	# Exit if version number hasn't been upped since last commit
	if [[ $lastReleaseVersionNumber == $currentVersionNumber ]] && ! [[ $headCommitHash == $lastReleaseCommitHash ]] && [[ -n $lastReleaseVersionNumber ]]
	then
		echo -e "   ${RED}[-] ERROR${RESET}: Version number in $src/main.go has not been bumped since last commit, exiting build"
		exit 1
	fi

    # Quick check for any left over debug prints
    if grep -ER "DEBUG" "$src"/*.go
    then
        echo -e "   ${YELLOW}[?] WARNING${RESET}: Debug print found in source code. You might want to remove that before release."
    fi

	# Quick staticcheck check - ignoring punctuation in error strings
	cd "$src"
	set +e
	staticcheck ./*.go | grep -Ev "error strings should not"
	set -e
	cd "$repoDir"/

	echo -e "   ${GREEN}[+] DONE${RESET}"
}

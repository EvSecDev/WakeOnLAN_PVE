#!/bin/bash
command -v git >/dev/null

function create_release_notes() {
	local repoDir localReleaseDir githubReleaseNotesFile lastReleaseCommitHash commitMsgsSinceLastRelease IFS commitMsg currentReleaseCommitHash
	repoDir=$1
	localReleaseDir=$2
	githubReleaseNotesFile="$localReleaseDir/release-notes.md"

	echo "[*] Retrieving all git commit messages since last release..."

	# Get commit where last release was generated from
	lastReleaseCommitHash=$(cat "$repoDir"/.last_release_commit)
	if [[ -z $lastReleaseCommitHash ]]
	then
		echo -e "${RED}[-] ERROR${RESET}: Could not determine when last release was by commit, refusing to continue" >&2
		exit 1
	fi

	# Collect commit messages up until the last release commit (not including the release commit messages
	commitMsgsSinceLastRelease=$(git log --format=%B "$lastReleaseCommitHash"~0..HEAD)

	if [[ -z $commitMsgsSinceLastRelease ]]
	then
		# Return early if HEAD is where last release was generated (no messages to format)
		echo -e "${RED}[-] ERROR${RESET}: No commits since last release" >&2
		exit 1
	fi

	# Format each commit message line by section
	IFS=$'\n'
	for commitMsg in $commitMsgsSinceLastRelease
	do
		# Skip empty lines
		if [[ -z $commitMsg ]]
		then
			continue
		fi

		# Parse out release message sections
		if echo "$commitMsg" | grep -qE "^[aA]dded"
		then
			comment_Added="$comment_Added$(echo "$commitMsg" | sed 's/^[ \t]*[aA]dded/\n -/g' | sed 's/^\([^a-zA-Z]*\)\([a-zA-Z]\)/\1\U\2/')"
		elif echo "$commitMsg" | grep -qE "^[cC]hanged"
		then
			comment_Changed="$comment_Changed$(echo "$commitMsg" | sed 's/^[ \t]*[cC]hanged/\n -/g' | sed 's/^\([^a-zA-Z]*\)\([a-zA-Z]\)/\1\U\2/')"
		elif echo "$commitMsg" | grep -qE "^[rR]emoved"
		then
			comment_Removed="$comment_Removed$(echo "$commitMsg" | sed 's/^[ \t]*[rR]emoved/\n -/g' | sed 's/^\([^a-zA-Z]*\)\([a-zA-Z]\)/\1\U\2/')"
		elif echo "$commitMsg" | grep -qE "^[fF]ixed"
		then
			comment_Fixed="$comment_Fixed$(echo "$commitMsg" | sed 's/^[ \t]*[fF]ixed/\n -/g' | sed 's/bug where //g' | sed 's/^\([^a-zA-Z]*\)\([a-zA-Z]\)/\1\U\2/')"
		else
			echo -e "   ${YELLOW}[?] WARNING${RESET}: UNSUPPORTED LINE PREFIX: '$commitMsg'"
		fi
	done

	# Release Notes Section headers
	local addedHeader changedHeader removedHeader fixedHeader trailerHeader trailerComment combinedMsg
	addedHeader="### :white_check_mark: Added"
	changedHeader="### :arrows_counterclockwise: Changed"
	removedHeader="### :x: Removed"
	fixedHeader="### :hammer: Fixed"
	trailerHeader="### :information_source: Instructions"
	trailerComment=" - Please refer to the README.md file for instructions"

	# Combine release notes sections
	combinedMsg=""
	if [[ -n $comment_Added ]]
	then
		combinedMsg="$addedHeader$comment_Added\n"
	fi
	if [[ -n $comment_Changed ]]
	then
		combinedMsg="$combinedMsg\n$changedHeader$comment_Changed\n"
	fi
	if [[ -n $comment_Removed ]]
	then
		combinedMsg="$combinedMsg\n$removedHeader$comment_Removed\n"
	fi
	if [[ -n $comment_Fixed ]]
	then
		combinedMsg="$combinedMsg\n$fixedHeader$comment_Fixed\n"
	fi

	# Add standard trailer
	combinedMsg="$combinedMsg\n$trailerHeader\n$trailerComment"

	# Save notes to file
	echo -e "$combinedMsg" > "$githubReleaseNotesFile"

	# Save commit that this release was made for to track file
	currentReleaseCommitHash=$(git show HEAD --pretty=format:"%H" --no-patch)
	echo "$currentReleaseCommitHash" > "$repoDir"/.last_release_commit

	echo "====================================================================="
	echo "RELEASE MESSAGE in $githubReleaseNotesFile - CHECK BEFORE PUBLISHING:"
	echo "====================================================================="
	echo -e "$combinedMsg"
	echo "====================================================================="
	echo "RELEASE ATTACHMENTS in $localReleaseDir"
	echo "====================================================================="
	find "$localReleaseDir"/ -maxdepth 1 -type f ! -iwholename "$githubReleaseNotesFile"
}

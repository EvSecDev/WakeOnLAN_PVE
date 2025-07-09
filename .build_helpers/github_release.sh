#!/bin/bash
command -v jq >/dev/null
command -v curl >/dev/null

function create_github_release() {
	local repoOwner githubRepo versionTag localReleaseDir githubReleaseNotesFile releaseNotes releaseMeta curlOutput releaseID finalReleaseURL preReleaseSet
	repoOwner=$1
	githubRepo=$2
	versionTag=$3

	if [[ -d $HOME/Downloads/releasetemp ]]
	then
		localReleaseDir="$HOME/Downloads/releasetemp"
	elif [[ -d $HOME/.local/tmp/releasetemp ]]
	then
		localReleaseDir="$HOME/.local/tmp/releasetemp"
	elif [[ -d /tmp/releasetemp ]]
	then
		localReleaseDir="/tmp/releasetemp"
	else
		echo -e "${RED}ERROR${RESET}: Unable to identify available temp dir, cannot continue" >&2
		exit 1
	fi

	githubReleaseNotesFile="$localReleaseDir/release-notes.md"

	if [[ -z $GITHUB_API_TOKEN ]]
	then
		echo -e "   ${RED}[-] ERROR${RESET}: GITHUB_API_TOKEN env variable is not set" >&2
		exit 1
	fi

	echo "[*] Creating new Github release with notes from file $githubReleaseNotesFile"

	releaseNotes=$(cat "$githubReleaseNotesFile")
	if [[ -z $releaseNotes ]]
	then
		echo -e "   ${RED}[-] ERROR${RESET}: Unable to read contents of release notes file $githubReleaseNotesFile" >&2
		exit 1
	fi

	# Escape newlines and carriage returns for inclusion in JSON
	releaseNotes=$(echo "$releaseNotes" | sed ':a;N;$!ba;s/\n/\\n/g')

	# Pre-release if major version number is 0
	if [[ $versionTag =~ ^v0\. ]]
	then
		preReleaseSet='true'
	else
		preReleaseSet='false'
	fi

	releaseMeta='{"tag_name":"'$versionTag'","target_commitish":"main","name":"","body":"'$releaseNotes'","draft":false,"prerelease":'$preReleaseSet',"generate_release_notes":false}'
	if ! jq . <<< "$releaseMeta" >/dev/null
	then
		echo -e "   ${RED}[-] ERROR${RESET}: Invalid release JSON, please check for unsupported characters in release notes" >&2
		exit 1
	fi

	curlOutput=$(curl --silent -L \
  -X POST \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer $GITHUB_API_TOKEN" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  -d "$releaseMeta" \
    'https://api.github.com/repos/'"$repoOwner"'/'"$githubRepo"'/releases')

	if [[ -z $curlOutput ]]
	then
		echo -e "   ${RED}[-] ERROR${RESET}: Received no response from github post to create release" >&2
		exit 1
	fi

	releaseID=$(jq -r .id <<< "$curlOutput")
	if [[ -z $releaseID ]] || [[ $releaseID == null ]]
	then
		errorResponse=$(jq -r .status <<< "$curlOutput")
		errorMessage=$(jq -r .message <<< "$curlOutput") 

		echo -e "   ${RED}[-] ERROR${RESET}: Unable to extract release ID from github response. ($errorResponse) $errorMessage" >&2
		exit 1
	fi

	finalReleaseURL=$(jq -r .url <<< "$curlOutput")

	echo -e "${GREEN}[+] Successfully${RESET} created new Github release - ID: $releaseID"
	rm "$githubReleaseNotesFile"

	cd "$localReleaseDir"
	if [[ $? != 0 ]]
	then
		echo -e "   ${RED}[-] ERROR${RESET}: Unable to cd into temp release dir" >&2
		exit 1
	fi

	# Upload every file that is not the notes in the temp dir
	local localFileName
	while IFS= read -r  -d '' localFileName
	do
		echo "  [*] Uploading file $localFileName to release $releaseID"

		curlOutput=$(curl --silent -L \
  	-X POST \
  	-H "Accept: application/vnd.github+json" \
  	-H "Authorization: Bearer $GITHUB_API_TOKEN" \
  	-H "X-GitHub-Api-Version: 2022-11-28" \
  	-H "Content-Type: application/octet-stream" \
  	--data-binary "@$localFileName" \
  	'https://uploads.github.com/repos/'"$repoOwner"'/'"$githubRepo"'/releases/'"$releaseID"'/assets?name='"$localFileName")

		if [[ -z $curlOutput ]]
		then
			echo -e "   ${RED}[-] ERROR${RESET}: Received no response from github post to upload attachments" >&2
			exit 1
		fi

		uploadState=$(jq -r .state <<< "$curlOutput")
		if [[ $uploadState != uploaded ]]  || [[ $uploadState == null ]]
		then
			echo -e "   ${RED}[-] ERROR${RESET}: Expected state to be uploaded but got $uploadState from github" >&2
			exit 1
		fi

		echo -e "  ${GREEN}[+] Successfully${RESET} uploaded file $localFileName to release $releaseID"
	done < <(find . -maxdepth 1 -type f -print0 | sed 's|\./||g')

	echo -e "${GREEN}[+]${RESET} Release published: $finalReleaseURL"

	# Cleanup
	rm -r "$localReleaseDir"
}

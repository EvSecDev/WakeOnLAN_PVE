#!/bin/bash

function update_go_packages {
	local repoDir src
	repoDir=$1
	src=$2

	cd "$repoDir/$src"
	if [[ $? != 0 ]]
	then
		echo -e "${RED}[-] ERROR${RESET}: Failed to move into source directory"
		return
	fi

	echo "[*] Updating Controller Go packages..."
	go get -u all
	if [[ $? != 0 ]]
	then
		echo -e "${RED}[-] ERROR${RESET}: Go module update failed"
		return
	fi

	go mod verify
	if [[ $? != 0 ]]
	then
		echo -e "${RED}[-] ERROR${RESET}: Go module verification failed"
		return
	fi

	go mod tidy
	echo -e "   ${GREEN}[+] DONE${RESET}"
}
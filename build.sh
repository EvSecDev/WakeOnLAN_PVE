#!/bin/bash
if [ -z "$BASH_VERSION" ]
then
	echo "This script must be run in BASH." >&2
	exit 1
fi

# Define colors - unsupported terminals fail safe
if [ -t 1 ] && { [[ "$TERM" =~ "xterm" ]] || [[ "$COLORTERM" == "truecolor" ]] || tput setaf 1 &>/dev/null; }
then
	readonly RED='\033[31m'
	readonly GREEN='\033[32m'
	readonly YELLOW='\033[33m'
	readonly BLUE='\033[34m'
	readonly RESET='\033[0m'
	readonly BOLD='\033[1m'
fi

readonly configFile="build.conf"
# shellcheck source=./build.conf
source "$configFile"
if [[ $? != 0 ]]
then
	echo -e "${RED}[-] ERROR${RESET}: Failed to import build config variables in $configFile" >&2
	exit 1
fi

# Bail on any failure
set -e

# Check for required commands
command -v go >/dev/null
command -v sha256sum >/dev/null

# Variables
repoRoot=$(pwd)

# Check for required external variables
if [[ -z $HOME ]]
then
	echo -e "${RED}[-] ERROR${RESET}: Missing HOME variable" >&2
	exit 1
fi
if [[ -z $repoRoot ]]
then
	echo -e "${RED}[-] ERROR${RESET}: Failed to determine current directory" >&2
	exit 1
fi

# Load external functions
while IFS= read -r -d '' helperFunction
do
	source "$helperFunction"
	if [[ $? != 0 ]]
	then
		echo -e "${RED}[-] ERROR${RESET}: Failed to import build helper functions" >&2
		exit 1
	fi
done < <(find .build_helpers/ -maxdepth 1 -type f -iname "*.sh" -print0)

##################################
# MAIN BUILD
##################################

function compile_program_prechecks() {
	# Always ensure we start in the root of the repository
	cd "$repoRoot"/

	# Check for things not supposed to be in a release
	if type	-t check_for_dev_artifacts &>/dev/null
	then
		check_for_dev_artifacts "$SRCdir" "$repoRoot"
	fi

	# Check for new packages that were imported but not included in version output
	if type -t update_program_package_imports &>/dev/null
	then
		update_program_package_imports "$repoRoot/$SRCdir" "$packagePrintLine"
	fi

	# Ensure readme has updated code blocks
	if type -t update_readme &>/dev/null
	then
		update_readme "$SRCdir" "$srcHelpMenuStartDelimiter" "$readmeHelpMenuStartDelimiter"
	fi
}

function compile_program() {
	local GOARCH GOOS buildFull replaceDeployedExe deployedBinaryPath buildVersion staticEnabled
	GOARCH=$1
	GOOS=$2
	buildFull=$3
	replaceDeployedExe=$4
	staticEnabled=$5

	# Move into dir
	cd $SRCdir

	# Run tests
	echo "[*] Running all tests..."
	go test
	echo -e "   ${GREEN}[+] DONE${RESET}"

	echo "[*] Compiling program binary..."

	# Vars for build
	export GOARCH
	export GOOS

	# Build binary
	if [[ $staticEnabled == true ]]
	then
		musl_build_static "$GOARCH" "$GOOS" "$repoRoot" "$outputEXE"
	else
		go build -o "$repoRoot"/"$outputEXE" -a -ldflags '-s -w -buildid= ' ./*.go
	fi
	cd "$repoRoot"

	# Get version
	buildVersion=$(./$outputEXE --versionid)

	# Rename to more descriptive if full build was requested
	if [[ $buildFull == true ]]
	then
		local fullNameEXE

		# Rename with version
		fullNameEXE="${outputEXE}_${buildVersion}_${GOOS}-${GOARCH}-static"
		mv "$outputEXE" "$fullNameEXE"

		# Create hash for built binary
		sha256sum "$fullNameEXE" > "$fullNameEXE".sha256
	elif [[ $replaceDeployedExe == true ]]
	then
		# Replace existing binary with new one
		deployedBinaryPath=$(which $outputEXE)
		if [[ -z $deployedBinaryPath ]]
		then
			echo -e "${RED}[-] ERROR${RESET}: Could not determine path of existing program binary, refusing to continue" >&2
			rm "$outputEXE"
			exit 1
		fi

		mv "$outputEXE" "$deployedBinaryPath"
	fi

	echo -e "   ${GREEN}[+] DONE${RESET}: Built version ${BOLD}${BLUE}$buildVersion${RESET}"
}

##################################
# START
##################################

function usage {
	echo "Usage $0
Program Build Script and Helpers

Options:
  -b           Build the program using defaults
  -r           Replace binary in path with updated one
  -a <arch>    Architecture of compiled binary (amd64, arm64) [default: amd64]
  -o <os>      Which operating system to build for (linux, windows) [default: linux]
  -u           Update go packages for program
  -s           Static build using musl docker
  -p           Prepare release notes and attachments
  -P <version> Publish release to github
  -h           Print this help menu
"
}

# DEFAULTS
architecture="amd64"
os="linux"

# Argument parsing
while getopts 'a:o:P:busprh' opt
do
	case "$opt" in
	  'a')
	    architecture="$OPTARG"
	    ;;
	  'b')
	    buildmode='true'
	    ;;
	  'r')
        replaceDeployedExe='true'
        ;;
	  'o')
	    os="$OPTARG"
	    ;;
	  'u')
	    updatepackages='true'
	    ;;
	  's')
	    buildStatic='true'
	    ;;
	  'p')
        prepareRelease='true'
        ;;
	  'P')
		publishVersion="$OPTARG"
		;;
	  'h')
	    usage
	    exit 0
 	    ;;
	  *)
	    usage
	    exit 0
 	    ;;
	esac
done

if [[ $prepareRelease == true ]]
then
	compile_program_prechecks
	compile_program "$architecture" "$os" 'true' 'false' 'true'
	tempReleaseDir=$(prepare_github_release_files "$fullNameProgramPrefix")
	create_release_notes "$repoRoot" "$tempReleaseDir"
elif [[ -n $publishVersion ]]
then
	create_github_release "$githubUser" "$githubRepoName" "$publishVersion"
elif [[ $updatepackages == true ]]
then
	update_go_packages "$repoRoot" "$SRCdir"
elif [[ $buildmode == true ]]
then
	compile_program_prechecks
	compile_program "$architecture" "$os" 'false' "$replaceDeployedExe" "$buildStatic"
else
	echo -e "${RED}ERROR${RESET}: Unknown option or combination of options" >&2
	exit 1
fi

exit 0

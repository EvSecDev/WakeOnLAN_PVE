#!/bin/bash
if [ -z "$BASH_VERSION" ]
then
	echo "This script must be run in BASH."
	exit 1
fi

# Bail on any failure
set -e

# Check for required commands
command -v go >/dev/null
command -v tar >/dev/null
command -v base64 >/dev/null
command -v sha256sum >/dev/null

# Vars
repoRoot=$(pwd)
sourceDir="src"

function usage {
	echo "Usage $0

Options:
  -a <arch>   Architecture of compiled binary (amd64, arm64) [default: amd64]
  -b <prog>   Which program to build (wolpve, wolpvepkg)
  -f          Build nicely named binary
"
}

function wolpve_binary {
	# Vars for build
	inputGoSource="*.go"
	outputEXE="wakeonlanserver"
	export GOARCH=$1
	export GOOS=$2

	# Build go binary - Google's gopacket pcap cannot be statically compiled
	cd $repoRoot/$sourceDir
	go build -o $repoRoot/$outputEXE -a -tags purego -ldflags '-s -w' $inputGoSource
	cd $repoRoot

	# Rename to more descriptive if full build was requested
	if [[ $3 == true ]]
	then
		# Get version
		version=$(./$outputEXE -v)
		wolpveEXE=""$outputEXE"_"$version"_$GOOS-$GOARCH-dynamic"

		# Rename with version
		mv $outputEXE $wolpveEXE
		sha256sum $wolpveEXE > "$wolpveEXE".sha256
	fi
}

function wolpve_package {
	# Vars for build
	inputGoSource="*.go"
	outputEXE="wakeonlanserver"
	export GOARCH=$1
	export GOOS=$2

	# Build go binary - Google's gopacket pcap cannot be statically compiled
        cd $repoRoot/$sourceDir
	go build -o $repoRoot/$outputEXE -a -tags purego -ldflags '-s -w' $inputGoSource
        cd $repoRoot

	# Get version
	version=$(./$outputEXE -v)
	wolpvePKG=""$outputEXE"_installer_"$version"_$GOOS-$GOARCH.sh"

	# Create install script
	tar -cvzf $outputEXE.tar.gz $outputEXE
	cp $repoRoot/install_wol.sh $wolpvePKG
	cat $outputEXE.tar.gz | base64 >> $wolpvePKG
	sha256sum $wolpvePKG > "$wolpvePKG".sha256

	# Cleanup
	rm $outputEXE.tar.gz $outputEXE
}

## START
# DEFAULT CHOICES
buildfull='false'
architecture="amd64"
os="linux"

# Argument parsing
while getopts 'a:b:o:fh' opt
do
	case "$opt" in
	  'a')
	    architecture="$OPTARG"
	    ;;
	  'b')
	    buildopt="$OPTARG"
	    ;;
	  'f')
	    buildfull='true'
	    ;;
	  'o')
	    os="$OPTARG"
	    ;;
	  'h')
	    echo "Unknown Option"
	    usage
	    exit 0
 	    ;;
      *)
	    echo "Unknown Option"
	    usage
	    exit 0
 	    ;;
	esac
done

if [[ $buildopt == wolpve ]]
then
	wolpve_binary "$architecture" "$os" "$buildfull"
	echo "Complete: wolpve binary built"
elif [[ $buildopt == wolpvepkg ]]
then
	wolpve_package "$architecture" "$os"
	echo "Complete: wolpve package built"
fi

exit 0

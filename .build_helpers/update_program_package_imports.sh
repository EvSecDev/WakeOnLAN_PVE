#!/bin/bash

function update_program_package_imports() {
	local searchDir printLineText mainFile allImports IFS pkg allPackages newPackagePrintLine
	searchDir=$1
	printLineText=$2

	echo "[*] Updating import package list in main source file..."

	# Hold cumulative (duplicated) imports from all go source files
	allImports=""

	while IFS= read -r -d '' gosrcfile
	do
        # Get space delimited single line list of imported package names (no quotes) for this go file
        allImports+=$(awk '/import \(/,/\)/' "$gosrcfile" | grep -Ev "import \(|\)|^\n$" | sed -e 's/"//g' -e 's/\s//g' | tr '\n' ' ' | sed 's/  / /g')
	done < <(find "$searchDir/" -maxdepth 1 -type f -iname "*.go" -print0)

	if [[ -z $allImports ]]
	then
		echo -e "   ${RED}[-] ERROR${RESET}: Package import search returned no results"
		exit 1
	fi

	# Put space delimited list of all the imports into an array
	IFS=' ' read -r -a pkgarr <<< "$allImports"

	# Create associative array for deduping
	declare -A packages

	# Add each import package to the associative array to delete dups
	for pkg in "${pkgarr[@]}"
	do
	        packages["$pkg"]=1
	done

	# Convert back to regular array
	allPackages=("${!packages[@]}")

	if [[ ${#allPackages[@]} == 0 ]]
	then
		echo -e "   ${RED}[-] ERROR${RESET}: Package import deduplication returned no results"
		exit 1
	fi

	# Format package list into go print line
	newPackagePrintLine=$'\t\t'"${printLineText}${allPackages[*]}"'\\n")'

	# Remove testing package
	newPackagePrintLine=${newPackagePrintLine// testing/}

	# Identify if there are no packages in the output
	if echo "$newPackagePrintLine" | grep -qE "^Direct Package Imports:\s+\\\n$"
	then
		echo -e "   ${RED}[-] ERROR${RESET}: New generated package import list is empty"
		exit 1
	fi

    mainFile=$(grep -il "func main() {" "$searchDir"/*.go | grep -Ev "testing")

	# Write new package line into go source file that has main function
	sed -i "/$printLineText/c\\$newPackagePrintLine" "$mainFile"

	echo -e "   ${GREEN}[+] DONE${RESET}"
}
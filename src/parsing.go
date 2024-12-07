// wakeonlanpve
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ###################################
//	MATCH MAC TO VM
// ###################################

// Finds matching MAC address in Proxmox VM configuration files and retrieves the VM ID, Type, and name
func matchMACtoVM(MACAddress string, VMConfigPaths []string) (VMID string, VMTYPE string, VMNAME string, err error) {
	// Recover from panic
	defer func() {
		if r := recover(); r != nil {
			logError("panic while processing received packet payload", fmt.Errorf("%v", r), false)
		}
	}()

	// RegEx vars
	VMNameRegex := regexp.MustCompile(`(?i)name\:\s(.*)`)

	// Search through VM config directories for a matching mac address from the packet payload
	for _, VMConfigPath := range VMConfigPaths {
		// Get a list of files in directory
		var configFiles []fs.DirEntry
		configFiles, err = os.ReadDir(VMConfigPath)
		if err != nil {
			fmt.Errorf("failed to walk VM config path %s: %v", VMConfigPath)
			return
		}

		// Search through files in this directory for matching MAC
		for _, dirEntry := range configFiles {
			// Skip sub-directories
			if dirEntry.IsDir() {
				continue
			}

			// Get name of file
			configFile := dirEntry.Name()

			// Skip files without .conf extension
			if !strings.HasSuffix(configFile, ".conf") {
				continue
			}

			// Get full path
			configFilePath := filepath.Join(VMConfigPath, configFile)

			// Read contents of this config file
			var configFileBytes []byte
			configFileBytes, err = os.ReadFile(configFilePath)
			if err != nil {
				err = fmt.Errorf(" %s: %v", configFilePath, err)
			}

			// Convert file contents to string
			configFileContents := string(configFileBytes)

			// Skip to next file if MAC isn't in this file
			if !strings.Contains(strings.ToUpper(configFileContents), MACAddress) {
				continue
			}

			// Found MAC match - add relevant VM info to variables to start VM
			VMID = strings.TrimSuffix(configFile, ".conf")
			VMTYPE = filepath.Base(filepath.Dir(configFilePath))
			VMNameLine := VMNameRegex.FindStringSubmatch(configFileContents)
			VMNAME = VMNameLine[1]
		}

		// Only error out if a VMID, VMTYPE was not found and err is present
		// This is to catch failed reads of config files, but only when the entire MAC search failed
		if err != nil && len(VMID) == 0 && VMTYPE == "" {
			err = fmt.Errorf("failed to read VM config(s):%v", err)
			return
		}
	}

	return
}

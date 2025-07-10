// wakeonlanpve
package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// ###################################
//	POWER ON VM
// ###################################

func powerOn(VMCMD string, TYPENAME string, VMID string, VMNAME string) (err error) {
	// Recover from panic
	defer func() {
		if r := recover(); r != nil {
			logError("panic while powering on VM", fmt.Errorf("%v", r), false)
		}
	}()

	// Check if VM is already running
	cmd := exec.Command(VMCMD, "status", VMID)
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("failed to check status of %s %s - %s: %v", TYPENAME, VMID, VMNAME, err)
		return
	}

	// Log and return if already running
	if strings.Contains(string(stdout), "running") {
		err = fmt.Errorf("already running: %s %s - %s", TYPENAME, VMID, VMNAME)
		return
	}

	// Start the VM based on VMID
	cmd = exec.Command(VMCMD, "start", VMID)
	_, err = cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("failed to start %s %s - %s: %v", TYPENAME, VMID, VMNAME, err)
		return
	}

	// Show progress to user
	logMessage("Powered on %s %s - %s", TYPENAME, VMID, VMNAME)
	return
}

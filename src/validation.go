// wakeonlanpve
package main

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/gopacket"
)

// ###################################
//	VALIDATE PACKET
// ###################################

// Ensures received packet payload is present, its length is correct, and its payload matches regex
// Extracts the first 12 hex characters from the payload
func validatePacket(recvPacket gopacket.Packet) (MACAddress string, err error) {
	// Get payload from packet - skip if empty
	payload := recvPacket.ApplicationLayer()
	if payload == nil {
		err = fmt.Errorf("payload is empty")
		return
	}

	// Convert payload to hex
	hexPayload := hex.EncodeToString(payload.Payload())

	// Ensure payload length is within bounds for a WOL packet
	if len(hexPayload) < 24 {
		err = fmt.Errorf("payload must be at least 24 characters long")
		return
	} else if len(hexPayload) > 205 {
		err = fmt.Errorf("payload cannot be more than 205 characters long")
		return
	}

	// Regex for a WOL packet payload
	MatchPayloadRegex := regexp.MustCompile("^(?:f{12})([0-9A-Fa-f]{2}){96}$")

	// Validate payload against mac address regex - skip if not mac address
	if !MatchPayloadRegex.MatchString(hexPayload) {
		err = fmt.Errorf("payload does not match mac address regex")
		return
	}

	// Trim preamble from WOL payload
	hexPayload = strings.TrimPrefix(hexPayload, "ffffffffffff")

	// Format MAC address from payload with colons
	for characterPosition := 0; characterPosition < 12; characterPosition += 2 {
		if characterPosition > 0 {
			MACAddress += ":"
		}
		MACAddress += strings.ToUpper(hexPayload[characterPosition : characterPosition+2])
	}

	return
}

// ###################################
//	VALIDATE VM INFORMATION
// ###################################

// Ensures VM info is not empty and matches expected format using regex
func validateVMInfo(VMID string, VMTYPE string, VMNAME string) (err error) {
	// Check for empty ID
	if VMID == "" {
		err = fmt.Errorf("could not find VM/LXC")
		return
	}

	// Check for empty type
	if VMTYPE == "" {
		err = fmt.Errorf("could not determine if VM or LXC")
		return
	}

	// Check for empty name
	if VMNAME == "" {
		err = fmt.Errorf("could not find VM/LXC name")
		return
	}

	// Sanity check received values for VM information
	// Validate VMID
	NumericRegex := regexp.MustCompile(`^[0-9]+$`)
	if !NumericRegex.MatchString(VMID) {
		err = fmt.Errorf("invalid VM ID (%s)", VMID)
		return
	}

	// Validate VM Type
	VirtualTypeRegex := regexp.MustCompile(`^(qemu-server|lxc)$`)
	if !VirtualTypeRegex.MatchString(VMTYPE) {
		err = fmt.Errorf("invalid VM Type (%s)", VMTYPE)
		return
	}

	// Validate VM Name
	HostnameRegex := regexp.MustCompile(`^([A-Za-z0-9-]{1,63}\.?)+[A-Za-z0-9-]{2,253}$`)
	if !HostnameRegex.MatchString(VMNAME) {
		err = fmt.Errorf("invalid VM Name (%s)", VMNAME)
		return
	}

	return
}

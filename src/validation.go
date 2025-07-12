// wakeonlanpve
package main

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/gopacket"
)

// ###################################
//	VALIDATE PACKET
// ###################################

// Ensures received packet payload is present, its length is correct, and its payload matches hexadecimal characters
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

	// Ensure payload length is expected WOL size
	if len(hexPayload) != 204 {
		err = fmt.Errorf("payload must be exactly 204 characters long")
		return
	}

	// Validate WOL packet payload prefix
	if !strings.HasPrefix(hexPayload, "ffffffffffff") {
		err = fmt.Errorf("payload does not have wakeonlan sync stream prefix")
		return
	}

	// Trim preamble from WOL payload
	hexPayload = strings.TrimPrefix(hexPayload, "ffffffffffff")

	// Validate rest of payload is only hex
	for _, char := range hexPayload {
		switch {
		case char >= '0' && char <= '9':
		case char >= 'a' && char <= 'f':
		case char >= 'A' && char <= 'F':
		default:
			err = fmt.Errorf("payload does not consist solely of hexadecimal characters")
			return
		}
	}

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

// Ensures VM info is not empty and matches expected format
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
	for _, char := range VMID {
		switch {
		case char >= '0' && char <= '9':
		default:
			err = fmt.Errorf("invalid VM ID (%s): ID does not consist solely of numeric characters (0-9)", VMID)
			return
		}
	}

	// Validate VM Type
	if VMTYPE != "qemu-server" && VMTYPE != "lxc" {
		err = fmt.Errorf("invalid VM Type (%s): must be 'qemu-server' or 'lxc'", VMTYPE)
		return
	}

	// Validate VM Name
	if len(VMNAME) > 255 {
		err = fmt.Errorf("invalid VM Name (%s): must not be more than 255 characters", VMNAME)
		return
	}
	for _, char := range VMNAME {
		switch {
		case char >= '0' && char <= '9':
		case char >= 'a' && char <= 'z':
		case char >= 'A' && char <= 'Z':
		case char == '-':
		case char == '.':
		default:
			err = fmt.Errorf("invalid VM Name (%s): must only contain alphanumeric, dash, or period characters", VMNAME)
			return
		}
	}

	return
}

// wakeonlanpve
package main

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

// ###################################
//	PROCESS PACKETS
// ###################################

func captureAndProcessPackets(WaitGroup *sync.WaitGroup, PCAPParameters ListenInterfaceParams, VMConfigPaths []string) {
	// Recover from panic
	defer func() {
		if r := recover(); r != nil {
			logError("panic while processing deployment", fmt.Errorf("%v", r), false)
		}
	}()

	defer WaitGroup.Done()

	// Open packet capture handle
	PCAPHandle, err := pcap.OpenLive(PCAPParameters.ListenIntf, 1600, PCAPParameters.PromiscMode, pcap.BlockForever)
	if err != nil {
		logError("failed to open capture device", err, false)
		return
	}
	defer PCAPHandle.Close()

	// Create BPF filter with parameters from config
	PCAPfilter := fmt.Sprintf("udp and ether src (%s) and src host (%s) and dst host (%s) and ether dst (%s) and dst port %s",
		strings.Join(PCAPParameters.FilterSrcMAC, " or "), strings.Join(PCAPParameters.FilterSrcIP, " or "),
		strings.Join(PCAPParameters.FilterDstIP, " or "), strings.Join(PCAPParameters.FilterDstMAC, " or "), PCAPParameters.FilterDstPort)

	err = PCAPHandle.SetBPFFilter(PCAPfilter)
	if err != nil {
		logError("failed to set BPF filter", err, false)
		return
	}

	logMessage(fmt.Sprintf("Listening for WOL packets on interface %s", PCAPParameters.ListenIntf))

	packetSource := gopacket.NewPacketSource(PCAPHandle, PCAPHandle.LinkType())
	for recvPacket := range packetSource.Packets() {
		// Ensure payload is valid and extract MAC address
		MACAddress, err := validatePacket(recvPacket)
		if err != nil {
			logMessage(fmt.Sprintf("Receivd invalid packet: %v", err))
			continue
		}

		// Log reception of WOL packet
		logMessage(fmt.Sprintf("Received Wake-on-LAN packet on interface %s", PCAPParameters.ListenIntf))

		// Get VM information from matching MAC
		VMID, VMTYPE, VMNAME, err := matchMACtoVM(MACAddress, VMConfigPaths)
		if err != nil {
			logMessage(fmt.Sprintf("Error searching for MAC Address: %v", err))
			continue
		}

		// Ensure VM information is valid
		err = validateVMInfo(VMID, VMTYPE, VMNAME)
		if err != nil {
			logMessage(fmt.Sprintf("Error: %v for MAC %s", err, MACAddress))
			continue
		}

		// Power on VM depending on type
		if strings.Contains(VMTYPE, "qemu") {
			err = powerOn("qm", "VM", VMID, VMNAME)
		} else if strings.Contains(VMTYPE, "lxc") {
			err = powerOn("pct", "LXC", VMID, VMNAME)
		}

		// Check for error in either power on function
		if err != nil {
			logMessage(fmt.Sprintf("%v", err))
			continue
		}
	}
}

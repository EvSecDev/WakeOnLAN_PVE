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

	logMessage("Setting capture filter as '%s'", PCAPfilter)

	err = PCAPHandle.SetBPFFilter(PCAPfilter)
	if err != nil {
		logError("failed to set BPF filter", err, false)
		return
	}

	logMessage("Listening for WOL packets on interface %s", PCAPParameters.ListenIntf)

	packetSource := gopacket.NewPacketSource(PCAPHandle, PCAPHandle.LinkType())
	for recvPacket := range packetSource.Packets() {
		// Get headers
		l2meta := recvPacket.LinkLayer().LinkFlow()
		l3meta := recvPacket.NetworkLayer().NetworkFlow()

		// Ensure payload is valid and extract MAC address
		MACAddress, err := validatePacket(recvPacket)
		if err != nil {
			logMessage("Receivd invalid packet from %s (%s): %v", l3meta.Src(), l2meta.Src(), err)
			continue
		}

		// Log reception of WOL packet
		logMessage("Received Wake-on-LAN packet on interface %s from %s (%s)", PCAPParameters.ListenIntf, l3meta.Src(), l2meta.Src())

		// Get VM information from matching MAC
		VMID, VMTYPE, VMNAME, err := matchMACtoVM(MACAddress, VMConfigPaths)
		if err != nil {
			logMessage("Error searching for MAC Address: %v", err)
			continue
		}

		// Ensure VM information is valid
		err = validateVMInfo(VMID, VMTYPE, VMNAME)
		if err != nil {
			logMessage("Error: %v for MAC %s", err, MACAddress)
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
			logMessage("%v", err)
			continue
		}
	}
}

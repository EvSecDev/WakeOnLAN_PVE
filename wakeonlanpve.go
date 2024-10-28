// wakeonlanpve
package main

import (
	"runtime"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log/syslog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

// ###################################
//      GLOBAL VARIABLES
// ###################################

type parseJsonConfig struct {
	ListenIntf            []ListenInterfaceParams `json:"listenIntf"`
	VMConfigPaths         []string                `json:"pathToVMConfigurations"`
	RemoteLogEnabled      bool		      `json:"syslogEnabled"`
	SyslogDestinationIP   string                  `json:"syslogDestinationIP"`
	SyslogDestinationPort string                  `json:"syslogDestinationPort"`
}

type ListenInterfaceParams struct {
	ListenIntf    string   `json:"listenIntf"`
	FilterSrcMAC  []string `json:"filterSrcMAC"`
	FilterSrcIP   []string `json:"filterSrcIP"`
	FilterDstIP   []string `json:"filterDstIP"`
	FilterDstMAC  []string `json:"filterDstMAC"`
	FilterDstPort string   `json:"filterDstPort"`
	PromiscMode   bool     `json:"PromiscuousMode"`
}

// For syslog messages
var remoteLogEnabled bool
var syslogAddress *net.UDPAddr

// ###################################
//      EXCEPTION HANDLING
// ###################################

func logError(errorDescription string, errorMessage error, exitRequested bool) {
	// Return early if no error
	if errorMessage == nil {
		return
	}

	// Create formatted error message and give to message func
	fullMessage := "Error: " + errorDescription + ": " + errorMessage.Error()
	logMessage(fullMessage)

	// Exit prog after sending error messages
	if exitRequested {
		os.Exit(1)
	}
}

func logMessage(message string) {
	var err error

	// Write to remote socket
	if remoteLogEnabled {
		err = logToRemote(message)
		if err == nil {
			return
		}
	}

	// Prep err from functions for writing to stdout
	if err != nil && err.Error() != "syslogAddress is empty" {
		message = "Failed to send message to desired location: " + err.Error() + " - ORIGINAL MESSAGE: " + message
	}

	// Write to stdout if other messages aren't selected or fail
	fmt.Printf("%s\n", message)
}

func logToRemote(message string) error {
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// If no address, go to stdout write
	if syslogAddress == nil {
		return fmt.Errorf("syslogAddress is empty")
	}

	// Open socket to remote syslog server
	conn, err := net.DialUDP("udp", nil, syslogAddress)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Format message
	syslogMsg := fmt.Sprintf("<%d>%s %s: %s", syslog.LOG_INFO, timestamp, "wol-server", message)

	// Write message to remote host - failure writes to stdout
	_, err = conn.Write([]byte(syslogMsg))
	if err != nil {
		return err
	}

	return nil
}

// ###################################
//	START HERE
// ###################################

func main() {
	progVersion := "v1.0.1"

	// GET CONFIGURATION PARAMETERS FROM JSON FILE
	var configFile string
	flag.StringVar(&configFile, "c", "wol-config.json", "Path to the configuration file")
	versionFlagExists := flag.Bool("V", false, "Print Version Information")
	versionNumberFlagExists := flag.Bool("v", false, "Print Version Information")
	flag.Parse()

	// VERSION INFO PRINTS
	if *versionFlagExists {
		fmt.Printf("WakeOnLAN_PVE %s compiled using %s(%s) on %s architecture %s\n", progVersion, runtime.Version(), runtime.Compiler, runtime.GOOS, runtime.GOARCH)
		fmt.Printf("First party packages: runtime encoding/hex encoding/json flag fmt log/syslog net os os/exec path/filepath regexp strings sync time\n")
		fmt.Printf("Third party packages: github.com/google/gopacket github.com/google/gopacket/pcap\n")
		os.Exit(0)
	}
	if *versionNumberFlagExists {
		fmt.Println(progVersion)
		os.Exit(0)
	}

	// LOAD CONFIG FILE CONTENTS
	jsonConfigFile, err := os.ReadFile(configFile)
	logError("failed to read config file", err, true)

	// PARSE CONFIG JSON
	var config parseJsonConfig
	err = json.Unmarshal(jsonConfigFile, &config)
	logError("failed to parse JSON config", err ,true)

	// SETUP REMOTE LOGGING
	if config.RemoteLogEnabled {
		// SET GLOBAL FOR LOG FUNC AWARENESS
		if strings.Contains(config.SyslogDestinationIP, ":") {
			syslogAddress, err = net.ResolveUDPAddr("udp", "["+config.SyslogDestinationIP+"]:"+config.SyslogDestinationPort)
		} else {
			syslogAddress, err = net.ResolveUDPAddr("udp", config.SyslogDestinationIP+":"+config.SyslogDestinationPort)
		}
		logError("failed to resolve syslog address", err, true)
		remoteLogEnabled = true
	} else {
		remoteLogEnabled = false
	}

	// SHOW PROGRESS
	logMessage("Server starting...")

	// SETUP AND START A PCAP FOR EACH INTERFACE SPECIFIED IN CONFIG FILE
	var WaitGroup sync.WaitGroup
	for _, intfParams := range config.ListenIntf {
		WaitGroup.Add(1)
		go captureAndProcessPackets(&WaitGroup, intfParams, config.VMConfigPaths)
	}
	WaitGroup.Wait()

	os.Exit(0)
}

// ###################################
//	PROCESS PACKETS
// ###################################

func captureAndProcessPackets(WaitGroup *sync.WaitGroup, PCAPParameters ListenInterfaceParams, VMConfigPaths []string) {
	// RECOVER FROM PANIC
	defer func() {
		if r := recover(); r != nil {
			logError("Controller panic while processing deployment", fmt.Errorf("%v", r), true)
		}
	}()

	// SIGNAL ROUTINE IS DONE WHEN FUNC RETURNS
	defer WaitGroup.Done()

	// REGEX VARS
	MatchPayloadRegex := regexp.MustCompile("ffffffffffff(([0-9A-Fa-f]{2}){6}){16}")
	VMNameRegex := regexp.MustCompile(`(?i)name\:\s(.*)`)

	// SETUP PACKET CAPTURE
	PCAPHandle, err := pcap.OpenLive(PCAPParameters.ListenIntf, 1600, PCAPParameters.PromiscMode, pcap.BlockForever)
	logError("failed to open capture device", err, true)
	defer PCAPHandle.Close()

	// DEFINE BPF FILTER FROM CONFIG OPTIONS
	PCAPfilter := fmt.Sprintf("udp and ether src (%s) and src host (%s) and dst host (%s) and ether dst (%s) and dst port %s",
		strings.Join(PCAPParameters.FilterSrcMAC, " or "), strings.Join(PCAPParameters.FilterSrcIP, " or "),
		strings.Join(PCAPParameters.FilterDstIP, " or "), strings.Join(PCAPParameters.FilterDstMAC, " or "), PCAPParameters.FilterDstPort)

	// SET BPF FILTER
	err = PCAPHandle.SetBPFFilter(PCAPfilter)
	logError("failed to set BPF filter", err ,true)

	// SHOW PROGRESS
	logMessage(fmt.Sprintf("Listening for WOL packets on interface %s", PCAPParameters.ListenIntf))

	// START PROCESSING PACKETS FROM LISTENING INTERFACE
	packetSource := gopacket.NewPacketSource(PCAPHandle, PCAPHandle.LinkType())
	for recvPacket := range packetSource.Packets() {
		// GET PAYLOAD FROM RECEIVED PACKET - SKIP IF NONE
		payload := recvPacket.ApplicationLayer()
		if payload == nil {
			continue
		}

		// CONVERT TO HEX
		hexPayload := hex.EncodeToString(payload.Payload())

		// ENSURE PAYLOAD LENGTH IS WITHIN BOUNDS FOR A WOL PACKET
		if len(hexPayload) < 24 {
			logMessage("Bad received packet: payload must be at least 24 characters long")
			continue
		} else if len(hexPayload) > 205 {
			logMessage("Bad received packet: payload cannot be more than 205 characters long")
			continue
		}

		// CHECK RECEIVED PAYLOAD AGAINST REGEX FOR MAC ADDR
		if !MatchPayloadRegex.MatchString(hexPayload) {
			logMessage("Bad received packet: payload does not match mac address regex")
			continue
		}

		// LOG RECEPTION OF WOL PACKET
		logMessage(fmt.Sprintf("Received Wake-on-LAN packet on interface %s", PCAPParameters.ListenIntf))

		// WAKE VM/LXC ASSOCIATED WITH RECEIVED MAC
		WakeVM(hexPayload[12:24], VMNameRegex, VMConfigPaths)
	}
}

// ###################################
//	PERFORM WOL ACTIONS
// ###################################

func WakeVM(payloadMACAddress string, VMNameRegex *regexp.Regexp, VMConfigPaths []string) {
	// CREATE FORMATTED MAC FROM PAYLOAD
	var MACAddress string
	for characterPosition := 0; characterPosition < 12; characterPosition += 2 {
		if characterPosition > 0 {
			MACAddress += ":"
		}
		MACAddress += strings.ToUpper(payloadMACAddress[characterPosition : characterPosition+2])
	}

	// LOOP THROUGH VMS AND FIND WHICH VM HAS MAC IN PACKET PAYLOAD
	var VMID, VMTYPE, VMNAME string
	for _, VMConfigPath := range VMConfigPaths {
		filepath.Walk(VMConfigPath, func(VMConfigPath string, info os.FileInfo, err error) error {
			logError(fmt.Sprintf("failed to walk VM config path %s", VMConfigPath), err, false)

			// SKIP TO NEXT FILE IF NOT .CONF EXTENSION
			if ! strings.HasSuffix(VMConfigPath, ".conf") {
				return nil
			}

			// READ IN CONTENTS FOR THIS FILE
			VMConfigFileContents, err := os.ReadFile(VMConfigPath)
			logError(fmt.Sprintf("failed to read VM config %s", VMConfigPath), err, false)

			// SKIP TO NEXT FILE IF MAC FROM PACKET PAYLOAD IS NOT PRESENT
			if ! strings.Contains(strings.ToUpper(string(VMConfigFileContents)), MACAddress) {
				return nil
			}

			// ASSIGN VM VARS USING INFO FROM VM/LXC CONFIG
			VMID = strings.TrimSuffix(filepath.Base(VMConfigPath), ".conf")
			VMTYPE = filepath.Base(filepath.Dir(VMConfigPath))
			VMNameLine := VMNameRegex.FindStringSubmatch(string(VMConfigFileContents))
			VMNAME = VMNameLine[1]
			return filepath.SkipDir
		})
	}

	// IF SEARCH DIDNT RETURN ANY RESULTS, NO VM HAS MATCHING MAC
	if len(VMID) == 0 {
		logMessage(fmt.Sprintf("Error: Could not find VM/LXC for MAC %v", MACAddress))
		return
	}

	// POWER ON VM OR LXC
	if strings.Contains(VMTYPE, "qemu") {
		logMessage(fmt.Sprintf("Powering on VM %s - %s", VMID, VMNAME))
		cmd := exec.Command("qm", "start", VMID)
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		logError(fmt.Sprintf("failed to start VM %s - %s", VMID, VMNAME), err, false)
	} else if strings.Contains(VMTYPE, "lxc") {
		logMessage(fmt.Sprintf("Powering on LXC %s - %s", VMID, VMNAME))
		cmd := exec.Command("pct", "start", VMID)
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		logError(fmt.Sprintf("failed to start LXC %s - %s", VMID, VMNAME), err, false)
	} else {
		logMessage(fmt.Sprintf("Error: could not determine if ID %s is a VM or LXC", VMID))
	}
}

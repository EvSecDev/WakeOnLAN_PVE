// wakeonlanpve
package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log/syslog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
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
	RemoteLogEnabled      bool                    `json:"syslogEnabled"`
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

// Program Meta Info
const progVersion = string("v1.0.2")
const usage = `
Options:
    -c, --config </path/to/json>    Path to the configuration file [default: wol-config.json]
    -s, --start-server              Start WOL Server (Requires '--config')
    -h, --help                      Show this help menu
    -V, --version                   Show version and packages
    -v, --versionid                 Show only version number

Documentation: <https://github.com/EvSecDev/WakeOnLAN_PVE>
`

// ###################################
//      EXCEPTION HANDLING
// ###################################

// Logs error description and error - will exit entire program if requested
func logError(errorDescription string, errorMessage error, exitRequested bool) {
	// Create formatted error message and give to message func
	fullMessage := "Error: " + errorDescription + ": " + errorMessage.Error()
	logMessage(fullMessage)

	// Exit prog after sending error messages
	if exitRequested {
		os.Exit(1)
	}
}

// Send message string to remote log server or stdout if remote log not enabled
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

// Sends message to remote syslog server in standard-ish format
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
//	START
// ###################################

func main() {
	// Program Argument Variables
	var configFile string
	var startServerFlagExists bool
	var versionFlagExists bool
	var versionNumberFlagExists bool

	// Read Program Arguments - allowing both short and long args
	flag.StringVar(&configFile, "c", "wol-config.json", "")
	flag.StringVar(&configFile, "config", "wol-config.json", "")
	flag.BoolVar(&startServerFlagExists, "s", false, "")
	flag.BoolVar(&startServerFlagExists, "start-server", false, "")
	flag.BoolVar(&versionFlagExists, "V", false, "")
	flag.BoolVar(&versionFlagExists, "version", false, "")
	flag.BoolVar(&versionNumberFlagExists, "v", false, "")
	flag.BoolVar(&versionNumberFlagExists, "versionid", false, "")

	// Custom help menu
	flag.Usage = func() { fmt.Printf("Usage: %s [OPTIONS]...\n%s", os.Args[0], usage) }
	flag.Parse()

	// Act on arguments
	if versionFlagExists {
		fmt.Printf("WakeOnLAN_PVE %s compiled using %s(%s) on %s architecture %s\n", progVersion, runtime.Version(), runtime.Compiler, runtime.GOOS, runtime.GOARCH)
		fmt.Print("Packages: runtime encoding/hex encoding/json flag fmt log/syslog net os os/exec path/filepath regexp strings sync time github.com/google/gopacket github.com/google/gopacket/pcap\n")
	} else if versionNumberFlagExists {
		fmt.Println(progVersion)
	} else if startServerFlagExists {
		err := startServer(configFile)
		if err != nil {
			logError("failed to start server", err, true)
		}
	}
}

// ###################################
//	PROCESS PACKETS
// ###################################

func startServer(configFile string) (err error) {
	// Load config file contents
	jsonConfigFile, err := os.ReadFile(configFile)
	if err != nil {
		err = fmt.Errorf("failed to read config file: %v", err)
		return
	}

	// Parse JSON config into struct
	var config parseJsonConfig
	err = json.Unmarshal(jsonConfigFile, &config)
	if err != nil {
		err = fmt.Errorf("failed to parse JSON config", err)
		return
	}

	// Setup remote logging if requested
	if config.RemoteLogEnabled {
		// Set address in global for awareness
		if strings.Contains(config.SyslogDestinationIP, ":") {
			syslogAddress, err = net.ResolveUDPAddr("udp", "["+config.SyslogDestinationIP+"]:"+config.SyslogDestinationPort)
		} else {
			syslogAddress, err = net.ResolveUDPAddr("udp", config.SyslogDestinationIP+":"+config.SyslogDestinationPort)
		}
		if err != nil {
			err = fmt.Errorf("failed to resolve syslog address", err)
			return
		}
		remoteLogEnabled = true
	} else {
		remoteLogEnabled = false
	}

	// SHOW PROGRESS
	logMessage(fmt.Sprintf("WOL-PVE Server (%s) starting...", progVersion))

	// Start packet captures for each listening interface
	if len(config.ListenIntf) == 1 {
		// If we are only listening on one interface, don't use a go routine (still have to use wait group)
		var WaitGroup sync.WaitGroup
		WaitGroup.Add(1)
		captureAndProcessPackets(&WaitGroup, config.ListenIntf[0], config.VMConfigPaths)
		WaitGroup.Wait()
	} else {
		// One go routine per listen interface
		var WaitGroup sync.WaitGroup
		for _, intfParams := range config.ListenIntf {
			WaitGroup.Add(1)
			go captureAndProcessPackets(&WaitGroup, intfParams, config.VMConfigPaths)
		}
		WaitGroup.Wait()
	}

	return
}

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

	// Signal when routine is done
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

	// Set BPF filter on capture handle
	err = PCAPHandle.SetBPFFilter(PCAPfilter)
	if err != nil {
		logError("failed to set BPF filter", err, false)
		return
	}

	// Show progress to user
	logMessage(fmt.Sprintf("Listening for WOL packets on interface %s", PCAPParameters.ListenIntf))

	// Process captured packets one at a time
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
	logMessage(fmt.Sprintf("Powered on %s %s - %s", TYPENAME, VMID, VMNAME))
	return
}

// wakeonlanpve
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
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
const progVersion string = "v1.0.3"
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

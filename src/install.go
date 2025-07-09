// wakeonlanpve
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func installServer() {
	if os.Geteuid() > 0 {
		fmt.Printf("Installation requires root permissions\n")
		return
	}

	const defaultExecutablePath string = "/usr/bin/wakeonlanserver-pve"
	const defaultConfigPath string = "/etc/wolpve-config.json"
	const defaultVMConfPaths string = "/etc/pve/local/qemu-server"
	const defaultLXCConfPaths string = "/etc/pve/local/lxc"

	const defaultListenIntf string = "lo"
	const defaultSrcMAC string = "00:50:56:11:22:33"
	const defaultSrcIP string = "127.0.0.1"
	const defaultDstIP string = "127.0.0.1"
	const defaultDstMac string = "00:00:00:00:00:00"
	const defaultDstPort string = "9"
	const defaultPromiscEnabled bool = false

	const defaultSyslogEnabled bool = false
	const defaultSyslogIP string = "127.0.0.1"
	const defaultSyslogPort string = "514"

	defaultApparmorProfilePath := "/etc/apparmor.d/" + strings.ReplaceAll(defaultExecutablePath, "/", ".")

	const defaultServiceDir = "/etc/systemd/system/"
	const defaultServiceName = "wakeonlanserver.service"

	defaultServiceFilePath := defaultServiceDir + defaultServiceName

	const defaultServiceUnit = `[Unit]
Description=Wake-on-LAN Server
After=network.target
StartLimitIntervalSec=1h
StartLimitBurst=6

[Service]
StandardOutput=journal
StandardError=journal
Type=simple
ExecStart=` + defaultExecutablePath + ` --start-server --config ` + defaultConfigPath + `
RestartSec=1min
Restart=always

[Install]
WantedBy=multi-user.target
`

	const defaultAAProfile = `### Apparmor Profile for the PVE WakeOnLAN Server
## This is a very locked down profile made for Debian systems
## Variables
@{exelocation}=` + defaultExecutablePath + `
@{configlocation}=` + defaultConfigPath + `

## Profile Begin
profile WOLPVE @{exelocation} flags=(enforce) {
  # Receive signals
  signal receive set=(int urg term kill exists cont),
  # Send signals to self
  signal send set=(int urg term exists) peer=WOLPVE,

  # Capabilities
  capability net_raw,
  network inet dgram,
  network inet6 dgram,
  network netlink raw,
  network packet raw,

  # Startup Configurations needed
  @{configlocation} r,

  # Allow execution of virtual machine cmd commands
  /usr/sbin/qm rmUx,
  /usr/sbin/pct rmUx,

  # etc access
  /etc/ld.so.cache r,
  /etc/pve/nodes/*/qemu-server/{,*} r,
  /etc/pve/nodes/*/lxc/{,*} r,
  ` + defaultVMConfPaths + `/* r,
  ` + defaultLXCConfPaths + `/* r,

  # sys access
  /sys/kernel/mm/transparent_hugepage/hpage_pmd_size r,
  /sys/devices/virtual/net/*/statistics/* r,

  # proc access
  owner /proc/[0-9]*/maps r,

  # usr access
  /usr/share/zoneinfo/** r,

  # dev access
  /dev/pts/* rw,
  /dev/null r,
}
`

	var defaultConfig Config
	defaultConfig.VMConfigPaths = []string{defaultVMConfPaths, defaultLXCConfPaths}

	var defaultListenParams ListenInterfaceParams
	defaultListenParams.PromiscMode = defaultPromiscEnabled

	_, err := promptUser("Press enter to start installation ")
	logError("Unable to create interactive prompt", err, true)

	userResponse, err := promptUser("Please enter the interface name to listen for wakeonlan packets: [default: %s]: ", defaultListenIntf)
	logError("Unable to create interactive prompt", err, true)

	if userResponse != "" {
		defaultListenParams.ListenIntf = strings.TrimSpace(userResponse)
	} else {
		defaultListenParams.ListenIntf = defaultListenIntf
	}

	userResponse, err = promptUser("Do you want to capture packets not addressed specifically for the interfaces address: [y/N]: ")
	logError("Unable to create interactive prompt", err, true)

	if strings.ToLower(userResponse) == "y" {
		defaultListenParams.PromiscMode = true
	}

	defaultListenParams.FilterDstIP = []string{defaultDstIP}
	defaultListenParams.FilterDstMAC = []string{defaultDstMac}
	defaultListenParams.FilterSrcIP = []string{defaultSrcIP}
	defaultListenParams.FilterSrcMAC = []string{defaultSrcMAC}

	userResponse, err = promptUser("Please enter the destination port number: [default: %s]: ", defaultDstPort)
	logError("Unable to create interactive prompt", err, true)

	if userResponse != "" {
		defaultListenParams.FilterDstPort = strings.TrimSpace(userResponse)
	} else {
		defaultListenParams.FilterDstPort = defaultDstPort
	}

	var config Config
	config.ListenIntf = []ListenInterfaceParams{defaultListenParams}
	config.VMConfigPaths = []string{defaultVMConfPaths, defaultLXCConfPaths}
	config.RemoteLogEnabled = defaultSyslogEnabled
	config.SyslogDestinationIP = defaultSyslogIP
	config.SyslogDestinationPort = defaultSyslogPort

	configBytes, err := json.MarshalIndent(config, "", "  ")
	logError("Failed to assemble JSON config", err, true)

	_, err = os.Stat(defaultConfigPath)
	if os.IsNotExist(err) {
		err = os.WriteFile(defaultConfigPath, configBytes, 0644)
		logError("Failed to write apparmor profile", err, true)

		fmt.Printf("Successfully installed configuration file at '%s'\n", defaultConfigPath)
	} else if err != nil {
		logError(fmt.Sprintf("Unable to check if an existing configuration file is located at '%s'", defaultConfigPath), err, true)
	}

	// Check if apparmor /sys path exists
	systemAAPath := "/sys/kernel/security/apparmor/profiles"
	_, err = os.Stat(systemAAPath)
	if os.IsNotExist(err) {
		fmt.Printf("AppArmor not supported by this system, not installing profile\n")
	} else if err != nil {
		logError("Unable to check if AppArmor is supported by this system", err, true)
	} else {
		// Write Apparmor Profile to /etc
		err = os.WriteFile(defaultApparmorProfilePath, []byte(defaultAAProfile), 0644)
		logError("Failed to write apparmor profile", err, true)

		// Enact Profile
		command := exec.Command("apparmor_parser", "-r", defaultApparmorProfilePath)
		_, err = command.CombinedOutput()
		logError("Failed to load apparmor profile", err, true)

		fmt.Printf("Successfully installed AppArmor Profile\n")
	}

	currentExePath := os.Args[0]
	if currentExePath != defaultExecutablePath {
		err = os.Rename(currentExePath, defaultExecutablePath)
		logError("Failed to move executable to default path", err, true)

		err = os.Chown(defaultExecutablePath, 0, 0)
		logError("Failed to set executable ownership to root", err, true)

		err = os.Chmod(defaultExecutablePath, 0755)
		logError("Failed to set executable permissions", err, true)
	}
	fmt.Printf("Successfully installed executable at '%s'\n", defaultExecutablePath)

	// Check if on systemd system
	systemdRunPath := "/run/systemd/system/"
	_, err = os.Stat(systemdRunPath)
	if os.IsNotExist(err) {
		fmt.Printf("Systemd not supported by this system, not installing systemd service\n")
	} else if err != nil {
		logError("Unable to check if Systemd is supported by this system\n", err, true)
	} else {
		err = os.WriteFile(defaultServiceFilePath, []byte(defaultServiceUnit), 0644)
		logError("Failed to write systemd service file", err, true)

		command := exec.Command("systemctl", "daemon-reload")
		_, err = command.CombinedOutput()
		logError("Failed to reload systemd unit configurations", err, true)

		command = exec.Command("systemctl", "enable", defaultServiceName)
		_, err = command.CombinedOutput()
		logError("Failed to set systemd service to start on boot", err, true)

		fmt.Printf("Successfully installed systemd service '%s' (Service is not started)\n", defaultServiceName)
	}

	fmt.Printf("Installation completed successfully. Don't forget to tweak the config file for your specific parameters (and start the service): %s\n", defaultConfigPath)
}

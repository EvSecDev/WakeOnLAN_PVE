#!/bin/bash
# Ensure script is run only in bash, required for built-ins (read, conditionals)
if [ -z "$BASH_VERSION" ]
then
	echo "This script must be run in BASH."
	exit 1
fi

# Only run script if running as root (or with sudo)
if [ "$EUID" -ne 0 ]
then
	echo "This script must be run with root permissions"
	exit 1
fi

#### Error handling

logError() {
	local errorMessage=$1
	local exitRequested=$2

	# print the error to the user
	echo "[-] Error: $errorMessage"

	if $exitRequested == "true"
	then
		exit 1
	fi
}

#### Default choices
executablePath="/usr/local/bin/wakeonlanserver"
configFilePath="/etc/wol-config.json"
vmConfigPath="/etc/pve/local/qemu-server"
lxcConfigPath="/etc/pve/local/lxc"
enableSyslog=false
syslogServerIP="127.0.0.1"
syslogServerPort="514"
ApparmorProfilePath=/etc/apparmor.d/$(echo $executablePath | sed 's|^/||g' | sed 's|/|.|g')
ServiceDir="/etc/systemd/system"
Service="wakeonlanserver.service"
ServiceFilePath="$ServiceDir/$Service"

#### Pre Checks

# Check for commands
command -v echo >/dev/null || logError "echo command not found." "true"
command -v egrep >/dev/null || logError "egrep command not found." "true"
command -v grep >/dev/null || logError "grep command not found." "true"
command -v tar >/dev/null || logError "tar command not found." "true"
command -v dirname >/dev/null || logError "dirname command not found." "true"
command -v mkdir >/dev/null || logError "mkdir command not found." "true"
command -v mv >/dev/null || logError "mv command not found." "true"
command -v rm >/dev/null || logError "rm command not found." "true"
command -v cat >/dev/null || logError "cat command not found." "true"
command -v base64 >/dev/null || logError "base64 command not found." "true"
command -v tail >/dev/null || logError "tail command not found." "true"
command -v ls >/dev/null || logError "ls command not found." "true"
command -v tr >/dev/null || logError "tr command not found." "true"
command -v awk >/dev/null || logError "awk command not found." "true"
command -v sed >/dev/null || logError "sed command not found." "true"
command -v print >/dev/null || logError "print command not found." "true"
command -v systemctl >/dev/null || logError "systemctl command not found." "true"

#### Installation
echo -e "\n ========================================"
echo "        WakeOnLAN_PVE Installer         "
echo "========================================"
read -p " Press enter to begin the installation"
echo -e "========================================"

# Put executable from local dir in user choosen location
PAYLOAD_LINE=$(awk '/^__PAYLOAD_BEGINS__/ { print NR + 1; exit 0; }' $0)
executableDirs=$(dirname $executablePath 2>/dev/null || logError "failed to determine executable parent directories" "true")
mkdir -p $executableDirs 2>/dev/null || logError "failed to create executable parent directory" "true"
tail -n +${PAYLOAD_LINE} $0 | base64 -d | tar -zpvx -C $executableDirs || logError "failed to extract embedded executable" "true"
chmod 755 $executablePath 2>/dev/null || logError "failed to change permissions of executable" "true"
chown root:root $executablePath 2>/dev/null || logError "failed to change ownership of executable" "true"
echo "[+] Successfully extracted wolpve binary to $executablePath"

# If service already exists, stop to allow new install over existing
if [[ -f $ServiceFilePath ]]
then
	systemctl stop $Service
fi

# Setup Systemd Service
cat > "$ServiceFilePath" <<EOF
[Unit]
Description=Wake-on-LAN Server
After=network.target
StartLimitIntervalSec=1h
StartLimitBurst=6

[Service]
StandardOutput=journal
StandardError=journal
Type=simple
ExecStart=$executablePath --start-server --config $configFilePath
RestartSec=1min
Restart=always

[Install]
WantedBy=multi-user.target
EOF
# reload units and enable
systemctl daemon-reload || logError "failed to reload systemd daemon for new unit" "true"
systemctl enable $Service || logError "failed to enable systemd service" "true"
echo "[+] Systemd service installed and enabled, start it with 'systemctl start $Service'"

# Install apparmor profile
cat > "$ApparmorProfilePath" <<EOF
### Apparmor Profile for the Secure Configuration Management Deployer SSH Server
## This is a very locked down profile made for Debian systems
## Variables
@{exelocation}=$executablePath
@{configlocation}=$configFilePath

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
  $vmConfigPath/* r,
  $lxcConfigPath/* r,

  # sys access
  /sys/kernel/mm/transparent_hugepage/hpage_pmd_size r,
  /sys/devices/virtual/net/*/statistics/* r,

  # proc access
  owner /proc/[0-9]*/maps r,

  # usr access
  /usr/share/zoneinfo/** r,

  # dev access
  /dev/null r,

  # Library access
  /usr/lib/*/libpcap.so.* rm,
  /usr/lib/*/libc.so.* rm,
  /usr/lib/*/libdbus-*.so.* rm,
  /usr/lib/*/libsystemd.so.* rm,
  /usr/lib/*/libcap.so.* rm,
  /usr/lib/*/libgcrypt.so.* rm,
  /usr/lib/*/liblzma.so.* rm,
  /usr/lib/*/libzstd.so.* rm,
  /usr/lib/*/liblz4.so.* rm,
  /usr/lib/*/libgpg-error.so.* rm,
}
EOF
chmod 644 "$ApparmorProfilePath" || logError "failed to change permissions of apparmor profile" "true"
chown root:root "$ApparmorProfilePath" || logError "failed to change ownership of apparmor profile" "true"
apparmor_parser -r "$ApparmorProfilePath" || logError "failed to enable apparmor profile" "true"
echo "[+] Successfully installed wolpve apparmor profile at $ApparmorProfilePath"

# Put config in user choosen location
cat > "$configFilePath" <<EOF
{
  "listenIntf": [
  {
    "listenIntf": "lo",
    "filterSrcMAC": ["00:50:56:11:22:33"],
    "filterSrcIP": ["127.0.0.1"],
    "filterDstIP": ["127.0.0.1"],
    "filterDstMAC": ["00:00:00:00:00:00"],
    "filterDstPort": "9",
    "PromiscuousMode": false
  },
  {
    "listenIntf": "vmbr0",
    "filterSrcMAC": ["00:50:56:11:44:55"],
    "filterSrcIP": ["192.168.10.5","192.168.10.2"],
    "filterDstIP": ["192.168.20.15"],
    "filterDstMAC": ["98:76:54:32:10:fe"],
    "filterDstPort": "9",
    "PromiscuousMode": true
  }
  ],
  "pathToVMConfigurations": ["$vmConfigPath","$lxcConfigPath"],
  "syslogEnabled": $enableSyslog,
  "syslogDestinationIP": "$syslogServerIP",
  "syslogDestinationPort": "$syslogServerPort"
}
EOF
echo "[+] Successfully created wolpve configuration at $configFilePath (please tweak it to your needs)"

echo "==== Finished Installation ===="
echo ""
echo "Don't forget to start the wakeonlanserver systemd service after configuring the config"
echo ""
exit 0

# WOLPVE Binary Embed #
__PAYLOAD_BEGINS__

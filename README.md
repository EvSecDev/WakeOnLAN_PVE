# WakeOnLAN PVE (Proxmox)
This program enables an administrator to use WakeOnLAN (WOL) utilities to power on virtual machines (VM) or Linux containers (LXC) via a standard WOL UDP port 9 packet.

This is based on the existing program here (credit to ojaksch): https://forum.proxmox.com/threads/update-wake-and-other-on-lan-for-vms-v0-3.26381/

### Overview

The way this is accomplished is by listening (via a packet capture) on an interface of the hypervisor for the standard WOL packet.
The packet capture BPF (Berkeley Packet Filter) and listen interface is the access control method, ensuring only authorized endpoints can power on VM/LXCs.
Once an authorized packet is received, the MAC address is decoded from UDP payload.
With the decoded MAC address, the program will attempt to find a match in the supplied paths to the individual VM/LXC configuration files. 
Once a VM match is found, it will use either the `qm` or `pct` commands to start the VM/LXC.

No special client is required for use with this program, any WOL client can be used provided that a few conditions are met.
 1. This program is set to listen on an interface, bridge, or tap that the WOL UDP packet will eventually enter/cross.
 2. The destination IP of the WOL client is reachable by the last hop network device, not including the hypervisor itself (yes, you can use an IP other than the broadcast address).
  - The common problem encountered here is that a router will refuse to transmit the packet if the MAC table does not have an entry for the destination IP.
 3. The WOL client is set wake the MAC address of the VM/LXC that you want to power on (not any other MAC).

With that said, this program is very flexible and allows for the following:
 - No need to change the firewall on the hypervisor, it listens passively for packets (in front of the firewall) since WOL UDP packets are unidirectional.
 - Destination IP for the WOL client is not important, so long as condition 1 above is met. This means you can send the WOL packet to any existing VM, with the WOL MAC set to some other powered off VM.
 - Powering on a VM from another VM inside the same subnet with ZERO access to the hypervisor's management IP or network interface.
 - Listening on multiple interfaces at the same time.
 - Filtering on many source/destination IPs per interface.

```
Usage of wol-server-pve-amd64-dynamic:
-V           Print Version Information
-c string    Path to the configuration file (default "wol-config.json")
```

### Deployment

1. Copy the wol-server binary to the hypervisor itself.
2. Copy the wol-config.json file to the hypervisor.
3. Configure the wol-config.json to include the options you require.
4. A system service file is supplied for persistence, if desired.
5. Start the service using the configuration JSON and test with your choice of WOL client program.

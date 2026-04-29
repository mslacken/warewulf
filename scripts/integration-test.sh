#!/bin/bash
set -ex

# Enable EPEL and CRB repositories
dnf -y install epel-release
dnf config-manager --set-enabled crb

# Install build and VM provisioning dependencies
dnf -y install \
	git \
	make \
	golang \
	gpgme-devel \
	python3-devel \
	qemu-kvm \
	edk2-ovmf \
	dnsmasq \
	iproute \
	iptables-nft \
	initscripts-service \
	yq \
	ipxe-bootimgs-x86 \
	ipxe-bootimgs-aarch64 \
	tftp-server \
	nfs-utils \
	policycoreutils-python-utils

# Build and install Warewulf
go mod vendor
make defaults PREFIX=/usr SYSCONFDIR=/etc
make build
make install
cp -f etc/warewulf.conf-el10 /etc/warewulf/warewulf.conf
systemctl daemon-reload

# Set up bridge and TAP interfaces for VMs
ip link add br0 type bridge
ip addr add 192.168.100.1/24 dev br0
ip link set br0 up

ip tuntap add tap0 mode tap
ip tuntap add tap1 mode tap
ip link set tap0 up
ip link set tap1 up
ip link set tap0 master br0
ip link set tap1 master br0

# Network configuration
export sms_ip=192.168.100.1
export internal_netmask=255.255.255.0
export internal_network=192.168.100.0
export eth_provision=br0

# Detect gateway and DNS from the host
export ipv4_gateway
ipv4_gateway=$(ip route | awk '/default/ {print $3; exit}')
export dns_servers
dns_servers=$(awk '/^nameserver/ {print $2; exit}' /etc/resolv.conf)

# Compute node definitions
export num_computes=2
c_ip[0]=192.168.100.100
c_ip[1]=192.168.100.101
c_mac[0]=52:54:00:00:01:00
c_mac[1]=52:54:00:00:01:01
c_name[0]=c0
c_name[1]=c1

# Create the tftpboot directory (not created by dnsmasq)
install -d -m 0755 /var/lib/tftpboot

semanage fcontext -a -t public_content_t "/var/lib/tftpboot(/.*)?"
restorecon -R -v /var/lib/tftpboot

# Configure warewulf.conf
yq -i '.ipaddr = "'"${sms_ip}"'"' /etc/warewulf/warewulf.conf
yq -i '.netmask = "'"${internal_netmask}"'"' /etc/warewulf/warewulf.conf
yq -i '.network = "'"${internal_network}"'"' /etc/warewulf/warewulf.conf
yq -i '.dhcp["range start"] = "'"${internal_network}"'"' \
	/etc/warewulf/warewulf.conf
yq -i '.dhcp["range end"] = "static"' /etc/warewulf/warewulf.conf
yq -i '.dhcp.template = "static"' /etc/warewulf/warewulf.conf

# Configure nodes.conf
sed -i "s/defaults,noauto,nofail,ro/defaults,nofail,ro/" \
	/etc/warewulf/nodes.conf

# Turn on debugging messages
yq -i '.nodeprofiles.default.kernel.args -= ["quiet"]' \
	/etc/warewulf/nodes.conf
echo "log-debug" >>/etc/dnsmasq.d/ww4-debug.conf

# Enable and start warewulfd
systemctl enable --now warewulfd

# Create profiles and overlays
wwctl profile add nodes --profile default --comment "Nodes profile"
wwctl overlay create nodeconfig
wwctl profile set --yes nodes --system-overlays nodeconfig \
	--runtime-overlays syncuser

# Set default network configuration
wwctl profile set -y nodes --netname=default --netdev="${eth_provision}"
wwctl profile set -y nodes --netname=default \
	--netmask="${internal_netmask}"
wwctl profile set -y nodes --netname=default \
	--gateway="${ipv4_gateway}"
wwctl profile set -y nodes --netname=default \
	--nettagadd=DNS="${dns_servers}"

# Configure all Warewulf services
wwctl configure --all

# Generate SSH keys
bash /etc/profile.d/ssh_setup.sh

# Import the base image
wwctl image import docker://ghcr.io/warewulf/warewulf-rockylinux:10 \
	rocky-10 --syncuser

# Add compute nodes
for ((i = 0; i < num_computes; i++)); do
	wwctl node add --image=rocky-10 --profile=nodes --netname=default \
		--ipaddr="${c_ip[$i]}" --hwaddr="${c_mac[$i]}" "${c_name[$i]}"
done

wwctl nodes set -y -A "crashkernel=no,net.ifnames=1,console=ttyS0,115200 earlyprintk=serial,ttyS0,115200" c[0-1]

# Rebuild image, overlays, and reconfigure
wwctl image build rocky-10
wwctl overlay build
wwctl configure --all

# Launch QEMU VMs in the background
# Pipe output through sed to strip ANSI escape sequences and prefix each line
/usr/libexec/qemu-kvm \
	-m 3048 -smp 2 -enable-kvm -boot n \
	-netdev tap,id=net0,ifname=tap0,script=no,downscript=no \
	-device virtio-net-pci,netdev=net0,mac="${c_mac[0]}" \
	-nographic -serial mon:stdio -cpu host 2>&1 |
	stdbuf -oL sed -u "s/\x1b\[[0-9;]*[a-zA-Z]//g; s/\r//g; s/^/[${c_name[0]}]: /" &
VM0_PID=$!

/usr/libexec/qemu-kvm \
	-m 3048 -smp 2 -enable-kvm -boot n \
	-netdev tap,id=net1,ifname=tap1,script=no,downscript=no \
	-device virtio-net-pci,netdev=net1,mac="${c_mac[1]}" \
	-nographic -serial mon:stdio -cpu host 2>&1 |
	stdbuf -oL sed -u "s/\x1b\[[0-9;]*[a-zA-Z]//g; s/\r//g; s/^/[${c_name[1]}]: /" &
VM1_PID=$!

echo "Started VMs with PIDs: ${VM0_PID}, ${VM1_PID}"

# Wait for VMs to become reachable via SSH
MAX_RETRIES=60
SLEEP_INTERVAL=10

for ((i = 0; i < num_computes; i++)); do
	echo "Waiting for ${c_name[$i]} (${c_ip[$i]}) to become reachable..."
	for ((try = 1; try <= MAX_RETRIES; try++)); do
		if ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 \
			"${c_ip[$i]}" hostname 2>/dev/null; then
			echo "${c_name[$i]} is up!"
			break
		fi
		echo "  Attempt ${try}/${MAX_RETRIES} - ${c_name[$i]} not ready yet"
		sleep "${SLEEP_INTERVAL}"
	done
	if [ "${try}" -gt "${MAX_RETRIES}" ]; then
		echo "ERROR: ${c_name[$i]} did not become reachable"
		exit 1
	fi
done

echo "All compute nodes are up and reachable."

# Verify OS on compute nodes
for ((i = 0; i < num_computes; i++)); do
	echo "OS release on ${c_name[$i]}:"
	ssh -o StrictHostKeyChecking=no "${c_ip[$i]}" cat /etc/os-release
done

# Shut down VMs
kill -9 "${VM0_PID}" "${VM1_PID}"

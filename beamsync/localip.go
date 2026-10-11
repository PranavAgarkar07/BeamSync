package beamsync

import (
	"net"
)

// GetLocalIP returns the host's LAN IP for building share URLs.
//
// Primary: a UDP "dial" to 8.8.8.8:80. No packet is sent; the kernel just
// selects the source address from the routing table (like `ip route get`).
// This works even with no internet as long as the Wi-Fi/hotspot interface
// has a default route.
//
// Fallback: interface enumeration (first up, non-loopback, global-unicast
// IPv4, skipping 169.254/16 link-local). This covers fully-offline hosts
// with no default route, where the dial fails and the old code returned
// 127.0.0.1 — a URL no peer can reach.
//
// NOTE: on Windows with Hyper-V/VPN/Docker virtual adapters, the dial may
// pick a vEthernet address. If the QR URL is unreachable, compare with
// `ipconfig` and use the Wi-Fi adapter's 192.168.x.x / 172.20.x.x address.
func GetLocalIP() string {
	if ip := dialOutIP(); ip != "" {
		return ip
	}
	if ip := firstUnicastIPv4(); ip != "" {
		return ip
	}
	return "127.0.0.1"
}

func dialOutIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP.IsLoopback() || addr.IP.IsUnspecified() {
		return ""
	}
	return addr.IP.String()
}

func firstUnicastIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip = ip.To4(); ip == nil || ip.IsLoopback() || !ip.IsGlobalUnicast() {
				continue
			}
			if ip[0] == 169 && ip[1] == 254 {
				continue // link-local, not routable to peers
			}
			return ip.String()
		}
	}
	return ""
}

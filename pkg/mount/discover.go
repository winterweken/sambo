package mount

import (
	"bufio"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// NFSExport represents an available NFS export from a server
type NFSExport struct {
	Server  string   // Server IP or hostname
	Path    string   // Export path
	Clients []string // Allowed clients (may be empty or contain "*")
}

// NFSServer represents a discovered NFS server
type NFSServer struct {
	IP      string
	Exports []NFSExport
}

// DiscoverNFSServers scans the local subnet for NFS servers using showmount
// Returns a list of servers that respond to showmount queries
func DiscoverNFSServers(subnet string, timeout time.Duration) ([]NFSServer, error) {
	// Parse the subnet (e.g., "192.168.10.0/24")
	_, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		return nil, fmt.Errorf("invalid subnet: %w", err)
	}

	// Generate list of IPs to scan
	var ips []net.IP
	for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); incIP(ip) {
		// Make a copy of the IP
		ipCopy := make(net.IP, len(ip))
		copy(ipCopy, ip)
		ips = append(ips, ipCopy)
	}

	// Skip network and broadcast addresses for /24 and larger
	if len(ips) > 2 {
		ips = ips[1 : len(ips)-1]
	}

	var servers []NFSServer
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrent scans
	semaphore := make(chan struct{}, 50)

	for _, ip := range ips {
		wg.Add(1)
		go func(ipAddr net.IP) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			ipStr := ipAddr.String()

			// Quick port check first (NFS typically on port 2049)
			if !isPortOpen(ipStr, 2049, timeout) {
				return
			}

			// Try showmount
			exports, err := GetNFSExports(ipStr)
			if err == nil && len(exports) > 0 {
				mu.Lock()
				servers = append(servers, NFSServer{
					IP:      ipStr,
					Exports: exports,
				})
				mu.Unlock()
			}
		}(ip)
	}

	wg.Wait()
	return servers, nil
}

// GetNFSExports queries a specific server for its NFS exports using showmount -e
func GetNFSExports(server string) ([]NFSExport, error) {
	cmd := exec.Command("showmount", "-e", server)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("showmount failed: %w", err)
	}

	return parseShowmountOutput(server, string(output))
}

// parseShowmountOutput parses the output of showmount -e
func parseShowmountOutput(server, output string) ([]NFSExport, error) {
	var exports []NFSExport

	scanner := bufio.NewScanner(strings.NewReader(output))

	// Skip header line "Exports list on <server>:"
	if scanner.Scan() {
		header := scanner.Text()
		if !strings.HasPrefix(header, "Exports list") {
			return nil, fmt.Errorf("unexpected showmount output format")
		}
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse line: "/path/to/export   client1 client2 ..."
		// The path is at the start, followed by whitespace and clients
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		export := NFSExport{
			Server: server,
			Path:   fields[0],
		}

		if len(fields) > 1 {
			export.Clients = fields[1:]
		}

		exports = append(exports, export)
	}

	return exports, scanner.Err()
}

// GetLocalSubnet returns the subnet of the primary network interface
func GetLocalSubnet() (string, error) {
	// Get all network interfaces
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range ifaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			// Skip IPv6 and loopback
			ip4 := ipNet.IP.To4()
			if ip4 == nil || ip4.IsLoopback() {
				continue
			}

			// Return the network in CIDR notation
			ones, _ := ipNet.Mask.Size()
			network := ip4.Mask(ipNet.Mask)
			return fmt.Sprintf("%s/%d", network.String(), ones), nil
		}
	}

	return "", fmt.Errorf("no suitable network interface found")
}

// incIP increments an IP address
func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// isPortOpen checks if a port is open on a host with timeout
func isPortOpen(host string, port int, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

package tool

import (
	"net"
	"os"
	"runtime"
	"strconv"
)

func ProcessNo() string {
	if os.Getpid() > 0 {
		return strconv.Itoa(os.Getpid())
	}
	return ""
}

func HostName() string {
	if hs, err := os.Hostname(); err == nil {
		return hs
	}
	return "unknown"
}

func OSName() string {
	return runtime.GOOS
}

func AllIPV4() (ipv4s []string) {
	adders, err := net.InterfaceAddrs()
	if err != nil {
		return
	}

	for _, addr := range adders {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				ipv4 := ipNet.IP.String()
				if ipv4 == "127.0.0.1" || ipv4 == "localhost" {
					continue
				}
				ipv4s = append(ipv4s, ipv4)
			}
		}
	}
	return
}

func IPV4() string {
	ipv4s := AllIPV4()
	if len(ipv4s) > 0 {
		return ipv4s[0]
	}
	return "no-hostname"
}

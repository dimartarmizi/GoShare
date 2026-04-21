package utils

import (
	"errors"
	"net"
	"sort"
)

func LocalIPv4Addrs() ([]string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	addrs := make([]string, 0, 4)
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		ifaceAddrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range ifaceAddrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP == nil {
				continue
			}

			v4 := ipNet.IP.To4()
			if v4 == nil {
				continue
			}
			addrs = append(addrs, v4.String())
		}
	}

	if len(addrs) == 0 {
		return nil, errors.New("no active IPv4 address found")
	}

	sort.Strings(addrs)
	return addrs, nil
}

func PrimaryIPv4() string {
	addrs, err := LocalIPv4Addrs()
	if err != nil || len(addrs) == 0 {
		return "127.0.0.1"
	}
	return addrs[0]
}

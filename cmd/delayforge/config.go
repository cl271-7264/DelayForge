package main

import (
	"fmt"
	"strings"
)

// BuildFilter compiles DamageConfig into a WinDivert filter string
func BuildFilter(cfg DamageConfig) string {
	var parts []string

	switch cfg.Direction {
	case "outbound":
		parts = append(parts, "outbound")
	case "inbound":
		parts = append(parts, "inbound")
	}

	switch cfg.Protocol {
	case "tcp":
		parts = append(parts, "tcp")
	case "udp":
		parts = append(parts, "udp")
	case "icmp":
		parts = append(parts, "icmp or icmpv6")
	}

	if cfg.PortFilter != "" {
		ports := strings.Split(cfg.PortFilter, ",")
		for i := range ports {
			ports[i] = strings.TrimSpace(ports[i])
		}
		if len(ports) == 1 {
			parts = append(parts, fmt.Sprintf("tcp.DstPort == %s or udp.DstPort == %s", ports[0], ports[0]))
		} else if len(ports) > 1 {
			var exprs []string
			for _, p := range ports {
				exprs = append(exprs, fmt.Sprintf("tcp.DstPort == %s or udp.DstPort == %s", p, p))
			}
			parts = append(parts, "("+strings.Join(exprs, " or ")+")")
		}
	}

	if cfg.IpFilter != "" {
		ips := strings.Split(cfg.IpFilter, ",")
		for _, ip := range ips {
			ip = strings.TrimSpace(ip)
			if ip != "" {
				parts = append(parts, fmt.Sprintf("ip.SrcAddr == %s or ip.DstAddr == %s", ip, ip))
			}
		}
	}

	if len(parts) == 0 {
		return "true"
	}
	return strings.Join(parts, " and ")
}

using DelayForge.Models;

namespace DelayForge.Engine;

/// <summary>
/// Compiles DamageConfig filter parameters into a WinDivert filter string.
/// </summary>
internal static class FilterCompiler
{
    /// <summary>
    /// Build a WinDivert filter string from the given config.
    /// </summary>
    public static string Compile(DamageConfig config)
    {
        var parts = new List<string>();

        // Direction
        switch (config.Direction)
        {
            case FilterDirection.Outbound:
                parts.Add("outbound");
                break;
            case FilterDirection.Inbound:
                parts.Add("inbound");
                break;
            case FilterDirection.Both:
                // no direction filter
                break;
        }

        // Protocol
        switch (config.Protocol)
        {
            case FilterProtocol.Tcp:
                parts.Add("tcp");
                break;
            case FilterProtocol.Udp:
                parts.Add("udp");
                break;
            case FilterProtocol.Icmp:
                parts.Add("icmp or icmpv6");
                break;
            case FilterProtocol.Any:
                // no protocol filter
                break;
        }

        // Port filter
        if (!string.IsNullOrWhiteSpace(config.PortFilter))
        {
            var ports = config.PortFilter
                .Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);
            if (ports.Length == 1)
            {
                parts.Add($"tcp.DstPort == {ports[0]} or udp.DstPort == {ports[0]}");
            }
            else if (ports.Length > 1)
            {
                var portExprs = ports.Select(p => $"tcp.DstPort == {p} or udp.DstPort == {p}");
                parts.Add("(" + string.Join(" or ", portExprs) + ")");
            }
        }

        // IP filter (simple CIDR / single IP)
        if (!string.IsNullOrWhiteSpace(config.IpFilter))
        {
            var ips = config.IpFilter
                .Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);
            foreach (var ip in ips)
            {
                if (ip.Contains('/'))
                {
                    // CIDR: convert to WinDivert network filter
                    parts.Add($"ip.SrcAddr == {ip} or ip.DstAddr == {ip}");
                }
                else
                {
                    parts.Add($"ip.SrcAddr == {ip} or ip.DstAddr == {ip}");
                }
            }
        }

        // Process filter — WinDivert doesn't directly filter by process name at the network layer.
        // Process filtering is handled at the engine level by inspecting the connection's owning process.

        // Default filter if nothing specified
        if (parts.Count == 0)
            return "true";

        return string.Join(" and ", parts);
    }
}

namespace DelayForge.Models;

/// <summary>
/// All network impairment parameters.
/// </summary>
public class DamageConfig
{
    /// <summary>Base delay in milliseconds added to every matching packet.</summary>
    public int LatencyMs { get; set; }

    /// <summary>Random jitter range ±ms added on top of base latency.</summary>
    public int JitterMs { get; set; }

    /// <summary>Probability [0..100] of dropping a matching packet.</summary>
    public double PacketLossPercent { get; set; }

    /// <summary>Probability [0..100] of sending a duplicate of a matching packet.</summary>
    public double DuplicatePercent { get; set; }

    /// <summary>Probability [0..100] of reordering a packet (send it after the next packet).</summary>
    public double ReorderPercent { get; set; }

    /// <summary>Probability [0..100] of flipping random bytes in the payload.</summary>
    public double TamperPercent { get; set; }

    /// <summary>Bandwidth limit in kbps. 0 = unlimited.</summary>
    public int ThrottleKbps { get; set; }

    // --- Filter parameters ---

    /// <summary>Process name filter (e.g. "chrome.exe"). Empty = all processes.</summary>
    public string ProcessFilter { get; set; } = string.Empty;

    /// <summary>IP address or CIDR filter (e.g. "192.168.1.0/24"). Empty = all IPs.</summary>
    public string IpFilter { get; set; } = string.Empty;

    /// <summary>Port filter (e.g. "443" or "80,443"). Empty = all ports.</summary>
    public string PortFilter { get; set; } = string.Empty;

    /// <summary>Protocol filter: Any, Tcp, Udp, Icmp.</summary>
    public FilterProtocol Protocol { get; set; } = FilterProtocol.Any;

    /// <summary>Direction filter: Both, Outbound, Inbound.</summary>
    public FilterDirection Direction { get; set; } = FilterDirection.Both;

    /// <summary>Create a deep clone for atomic swap.</summary>
    public DamageConfig Clone()
    {
        return (DamageConfig)MemberwiseClone();
    }
}

public enum FilterProtocol
{
    Any,
    Tcp,
    Udp,
    Icmp
}

public enum FilterDirection
{
    Both,
    Outbound,
    Inbound
}

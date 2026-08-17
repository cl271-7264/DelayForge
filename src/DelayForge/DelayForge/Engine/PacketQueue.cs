using System.Collections.Concurrent;

namespace DelayForge.Engine;

/// <summary>
/// A timestamped packet waiting to be sent after a delay.
/// </summary>
internal sealed class DelayedPacket
{
    public byte[] Data { get; init; } = Array.Empty<byte>();
    public WinDivertInterop.WINDIVERT_ADDRESS Address { get; init; }
    public long DeadlineTicks { get; init; }
    public bool IsDuplicate { get; init; }
}

/// <summary>
/// Thread-safe priority queue for delayed/reordered packets.
/// Packets are ordered by their send deadline.
/// Uses a sorted concurrent bag with lock-free peek where possible.
/// </summary>
internal sealed class PacketDelayQueue
{
    private readonly object _lock = new();
    private readonly SortedList<long, DelayedPacket> _queue = new();

    public int Count
    {
        get { lock (_lock) return _queue.Count; }
    }

    public void Enqueue(DelayedPacket packet)
    {
        lock (_lock)
        {
            // Handle deadline collisions by slightly offsetting
            long key = packet.DeadlineTicks;
            while (_queue.ContainsKey(key))
                key++;
            _queue[key] = packet;
        }
    }

    /// <summary>
    /// Dequeue all packets whose deadline has passed.
    /// </summary>
    public List<DelayedPacket> DequeueReady()
    {
        var ready = new List<DelayedPacket>();
        long now = Environment.TickCount64;

        lock (_lock)
        {
            while (_queue.Count > 0)
            {
                var first = _queue.Keys[0];
                if (first > now)
                    break;
                ready.Add(_queue[first]);
                _queue.RemoveAt(0);
            }
        }
        return ready;
    }

    public void Clear()
    {
        lock (_lock)
        {
            _queue.Clear();
        }
    }
}

/// <summary>
/// Token bucket for bandwidth throttling.
/// </summary>
internal sealed class TokenBucket
{
    private double _tokens;
    private readonly object _lock = new();
    private long _lastRefillTick;

    public long CurrentQueueDepth { get; private set; }

    public void Configure(int kbps)
    {
        lock (_lock)
        {
            _tokens = 0;
            _lastRefillTick = Environment.TickCount64;
        }
    }

    /// <summary>
    /// Try to consume tokens for a packet of given size.
    /// Returns true if packet can be sent now, false if it should be queued.
    /// </summary>
    public bool TryConsume(int packetSize, int throttleKbps)
    {
        if (throttleKbps <= 0) return true;

        double maxTokens = throttleKbps * 1024.0 / 8.0; // bytes per second
        double bytesPerMs = maxTokens / 1000.0;

        lock (_lock)
        {
            long now = Environment.TickCount64;
            long elapsed = now - _lastRefillTick;
            _lastRefillTick = now;

            // Refill tokens
            _tokens = Math.Min(maxTokens, _tokens + elapsed * bytesPerMs);

            if (_tokens >= packetSize)
            {
                _tokens -= packetSize;
                return true;
            }
            return false;
        }
    }

    public void Reset()
    {
        lock (_lock)
        {
            _tokens = 0;
            _lastRefillTick = Environment.TickCount64;
        }
    }
}

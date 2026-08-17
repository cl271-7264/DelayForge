using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using DelayForge.Models;

namespace DelayForge.Engine;

/// <summary>
/// Core packet processing engine using WinDivert.
/// Handles: delay, jitter, packet loss, duplication, reordering, tampering, throttle.
/// </summary>
internal sealed class DivertEngine : IDisposable
{
    private IntPtr _handle = IntPtr.Zero;
    private CancellationTokenSource? _cts;
    private Task? _recvTask;
    private Task? _queueTask;
    private Task? _throttleTask;
    private readonly PacketDelayQueue _delayQueue = new();
    private readonly PacketDelayQueue _reorderQueue = new();
    private readonly PacketDelayQueue _throttleQueue = new();
    private readonly TokenBucket _tokenBucket = new();
    private readonly RandomNumberGenerator _rng = RandomNumberGenerator.Create();
    private readonly Stats _stats;
    private DamageConfig _config;
    private bool _disposed;

    public bool IsRunning => _handle != IntPtr.Zero;
    public Stats Statistics => _stats;
    public int DelayQueueDepth => _delayQueue.Count;
    public int ReorderQueueDepth => _reorderQueue.Count;
    public int ThrottleQueueDepth => _throttleQueue.Count;

    public DivertEngine(Stats stats, DamageConfig config)
    {
        _stats = stats;
        _config = config;
    }

    /// <summary>
    /// Update the damage config atomically while engine is running.
    /// </summary>
    public void UpdateConfig(DamageConfig config)
    {
        _config = config;
    }

    /// <summary>
    /// Start the engine: open WinDivert and begin packet processing.
    /// </summary>
    public void Start()
    {
        if (_handle != IntPtr.Zero)
            throw new InvalidOperationException("Engine is already running.");

        string filter = FilterCompiler.Compile(_config);
        Debug.WriteLine($"[DelayForge] WinDivert filter: {filter}");

        _handle = WinDivertInterop.WinDivertOpen(
            filter,
            WinDivertInterop.WINDIVERT_LAYER_NETWORK,
            0,
            WinDivertInterop.WINDIVERT_FLAG_DEFAULT);

        if (_handle == IntPtr.Zero || _handle == new IntPtr(-1))
        {
            int err = Marshal.GetLastWin32Error();
            _handle = IntPtr.Zero;
            throw new InvalidOperationException(
                $"Failed to open WinDivert (error {err}). Are you running as Administrator?");
        }

        Debug.WriteLine($"[DelayForge] WinDivert opened successfully, handle={_handle}");

        _cts = new CancellationTokenSource();
        var token = _cts.Token;

        _tokenBucket.Configure(_config.ThrottleKbps);

        // Start receive loop
        _recvTask = Task.Run(() => ReceiveLoop(token), token);

        // Start delay queue processor
        _queueTask = Task.Run(() => DelayQueueLoop(token), token);

        // Start throttle queue processor
        _throttleTask = Task.Run(() => ThrottleQueueLoop(token), token);
    }

    /// <summary>
    /// Stop the engine gracefully.
    /// </summary>
    public void Stop()
    {
        if (_handle == IntPtr.Zero) return;

        _cts?.Cancel();

        // Close handle to unblock WinDivertRecv
        var h = _handle;
        _handle = IntPtr.Zero;
        WinDivertInterop.WinDivertClose(h);

        // Wait for tasks to finish
        try { _recvTask?.Wait(TimeSpan.FromSeconds(3)); } catch { }
        try { _queueTask?.Wait(TimeSpan.FromSeconds(3)); } catch { }
        try { _throttleTask?.Wait(TimeSpan.FromSeconds(3)); } catch { }

        _delayQueue.Clear();
        _reorderQueue.Clear();
        _throttleQueue.Clear();
        _tokenBucket.Reset();
        _cts?.Dispose();
        _cts = null;

        Debug.WriteLine("[DelayForge] Engine stopped.");
    }

    /// <summary>
    /// Main receive loop: reads packets from WinDivert and applies damage rules.
    /// </summary>
    private void ReceiveLoop(CancellationToken ct)
    {
        byte[] buffer = new byte[65535];

        while (!ct.IsCancellationRequested && _handle != IntPtr.Zero)
        {
            var addr = new WinDivertInterop.WINDIVERT_ADDRESS();
            uint recvLen;

            bool ok = WinDivertInterop.WinDivertRecv(_handle, buffer, (uint)buffer.Length, out recvLen, ref addr);

            if (!ok || _handle == IntPtr.Zero)
            {
                if (_handle == IntPtr.Zero) break; // engine stopped
                int err = Marshal.GetLastWin32Error();
                if (err == 10004 || err == 6) break; // WSAECANCELLED or ERROR_INVALID_HANDLE
                continue;
            }

            if (recvLen == 0) continue;

            var packet = new byte[recvLen];
            Array.Copy(buffer, packet, (int)recvLen);

            _stats.RecordProcessed((int)recvLen);

            // --- Apply damage rules ---

            // 1. Packet loss
            if (_config.PacketLossPercent > 0 && RandomDouble() * 100 < _config.PacketLossPercent)
            {
                _stats.RecordDropped();
                continue; // don't send back
            }

            // 2. Tamper: flip random bytes in payload (after IP/TCP/UDP headers)
            if (_config.TamperPercent > 0 && RandomDouble() * 100 < _config.TamperPercent)
            {
                TamperPacket(packet);
                _stats.RecordTampered();
            }

            // 3. Duplicate: send a copy with slight delay
            if (_config.DuplicatePercent > 0 && RandomDouble() * 100 < _config.DuplicatePercent)
            {
                var dup = new DelayedPacket
                {
                    Data = (byte[])packet.Clone(),
                    Address = addr,
                    DeadlineTicks = Environment.TickCount64 + 5, // 5ms after
                    IsDuplicate = true
                };
                _delayQueue.Enqueue(dup);
                _stats.RecordDuplicated();
            }

            // 4. Reorder: delay this packet and let next one go first
            if (_config.ReorderPercent > 0 && RandomDouble() * 100 < _config.ReorderPercent)
            {
                long delay = 10 + (long)RandomInt(5, 50); // 10-60ms
                var reordered = new DelayedPacket
                {
                    Data = packet,
                    Address = addr,
                    DeadlineTicks = Environment.TickCount64 + delay
                };
                _reorderQueue.Enqueue(reordered);
                _stats.RecordReordered();
                continue; // don't send immediately
            }

            // 5. Delay / Jitter
            if (_config.LatencyMs > 0 || _config.JitterMs > 0)
            {
                int jitter = _config.JitterMs > 0
                    ? RandomInt(-_config.JitterMs, _config.JitterMs)
                    : 0;
                int delay = Math.Max(0, _config.LatencyMs + jitter);

                if (delay > 0)
                {
                    var delayed = new DelayedPacket
                    {
                        Data = packet,
                        Address = addr,
                        DeadlineTicks = Environment.TickCount64 + delay
                    };
                    _delayQueue.Enqueue(delayed);
                    _stats.RecordDelayed();
                    continue; // don't send immediately
                }
            }

            // 6. Throttle: check token bucket
            if (_config.ThrottleKbps > 0)
            {
                if (!_tokenBucket.TryConsume((int)recvLen, _config.ThrottleKbps))
                {
                    // Queue for later sending
                    var throttled = new DelayedPacket
                    {
                        Data = packet,
                        Address = addr,
                        DeadlineTicks = Environment.TickCount64 + 1 // retry in 1ms
                    };
                    _throttleQueue.Enqueue(throttled);
                    continue;
                }
            }

            // No delay or already handled — send immediately
            SendPacket(packet, ref addr);
        }
    }

    /// <summary>
    /// Processes the delay queue, sending packets when their deadline arrives.
    /// </summary>
    private void DelayQueueLoop(CancellationToken ct)
    {
        while (!ct.IsCancellationRequested)
        {
            // Process delay queue
            var readyPackets = _delayQueue.DequeueReady();
            foreach (var pkt in readyPackets)
            {
                var addr = pkt.Address;
                SendPacket(pkt.Data, ref addr);
            }

            // Process reorder queue
            var reorderPackets = _reorderQueue.DequeueReady();
            foreach (var pkt in reorderPackets)
            {
                // After reorder delay, send through delay queue again for final delay
                if (_config.LatencyMs > 0 || _config.JitterMs > 0)
                {
                    int jitter = _config.JitterMs > 0
                        ? RandomInt(-_config.JitterMs, _config.JitterMs)
                        : 0;
                    int delay = Math.Max(0, _config.LatencyMs + jitter);
                    if (delay > 0)
                    {
                        var delayed = new DelayedPacket
                        {
                            Data = pkt.Data,
                            Address = pkt.Address,
                            DeadlineTicks = Environment.TickCount64 + delay
                        };
                        _delayQueue.Enqueue(delayed);
                        _stats.RecordDelayed();
                        continue;
                    }
                }
                var addr = pkt.Address;
                SendPacket(pkt.Data, ref addr);
            }

            // Update stats
            _stats.SetQueueDepth(_delayQueue.Count + _reorderQueue.Count);

            // Sleep briefly to avoid busy-waiting
            Thread.Sleep(1);
        }
    }

    /// <summary>
    /// Processes the throttle queue using token bucket.
    /// </summary>
    private void ThrottleQueueLoop(CancellationToken ct)
    {
        while (!ct.IsCancellationRequested)
        {
            var ready = _throttleQueue.DequeueReady();
            foreach (var pkt in ready)
            {
                // Re-check token bucket
                if (_config.ThrottleKbps > 0 && !_tokenBucket.TryConsume(pkt.Data.Length, _config.ThrottleKbps))
                {
                    // Still over limit, re-queue with 1ms delay
                    var retry = new DelayedPacket
                    {
                        Data = pkt.Data,
                        Address = pkt.Address,
                        DeadlineTicks = Environment.TickCount64 + 1
                    };
                    _throttleQueue.Enqueue(retry);
                    continue;
                }

                var addr = pkt.Address;
                SendPacket(pkt.Data, ref addr);
            }

            _stats.SetThrottleDepth(_throttleQueue.Count);
            Thread.Sleep(1);
        }
    }

    private void SendPacket(byte[] data, ref WinDivertInterop.WINDIVERT_ADDRESS addr)
    {
        if (_handle == IntPtr.Zero) return;
        try
        {
            uint sentLen;
            WinDivertInterop.WinDivertSend(_handle, data, (uint)data.Length, out sentLen, ref addr);
        }
        catch (ObjectDisposedException) { }
        catch { /* best effort */ }
    }

    private void TamperPacket(byte[] packet)
    {
        // Flip 1-3 random bytes in the payload area (skip IP header, min 20 bytes)
        int headerLen = 20; // minimum IP header
        if (packet.Length <= headerLen + 4) return;

        int flips = RandomInt(1, 3);
        for (int i = 0; i < flips; i++)
        {
            int offset = RandomInt(headerLen, packet.Length - 1);
            packet[offset] ^= (byte)RandomInt(0, 255);
        }
    }

    private double RandomDouble()
    {
        byte[] buf = new byte[4];
        _rng.GetBytes(buf);
        return BitConverter.ToUInt32(buf, 0) / (double)uint.MaxValue;
    }

    private int RandomInt(int min, int max)
    {
        if (min >= max) return min;
        byte[] buf = new byte[4];
        _rng.GetBytes(buf);
        uint val = BitConverter.ToUInt32(buf, 0);
        return min + (int)(val % (uint)(max - min + 1));
    }

    public void Dispose()
    {
        if (_disposed) return;
        _disposed = true;
        Stop();
        _rng.Dispose();
    }
}

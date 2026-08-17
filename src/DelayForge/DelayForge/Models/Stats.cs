using System.Threading;

namespace DelayForge.Models;

/// <summary>
/// Thread-safe packet processing statistics.
/// </summary>
public class Stats
{
    private long _totalProcessed;
    private long _totalDelayed;
    private long _totalDropped;
    private long _totalDuplicated;
    private long _totalReordered;
    private long _totalTampered;
    private long _totalBytes;
    private long _currentQueueDepth;
    private long _throttleQueueDepth;

    public long TotalProcessed => Interlocked.Read(ref _totalProcessed);
    public long TotalDelayed => Interlocked.Read(ref _totalDelayed);
    public long TotalDropped => Interlocked.Read(ref _totalDropped);
    public long TotalDuplicated => Interlocked.Read(ref _totalDuplicated);
    public long TotalReordered => Interlocked.Read(ref _totalReordered);
    public long TotalTampered => Interlocked.Read(ref _totalTampered);
    public long TotalBytes => Interlocked.Read(ref _totalBytes);
    public long CurrentQueueDepth => Interlocked.Read(ref _currentQueueDepth);
    public long ThrottleQueueDepth => Interlocked.Read(ref _throttleQueueDepth);

    public void RecordProcessed(int bytes) { Interlocked.Increment(ref _totalProcessed); Interlocked.Add(ref _totalBytes, bytes); }
    public void RecordDelayed() { Interlocked.Increment(ref _totalDelayed); }
    public void RecordDropped() { Interlocked.Increment(ref _totalDropped); }
    public void RecordDuplicated() { Interlocked.Increment(ref _totalDuplicated); }
    public void RecordReordered() { Interlocked.Increment(ref _totalReordered); }
    public void RecordTampered() { Interlocked.Increment(ref _totalTampered); }
    public void SetQueueDepth(long depth) { Interlocked.Exchange(ref _currentQueueDepth, depth); }
    public void SetThrottleDepth(long depth) { Interlocked.Exchange(ref _throttleQueueDepth, depth); }

    public void Reset()
    {
        Interlocked.Exchange(ref _totalProcessed, 0);
        Interlocked.Exchange(ref _totalDelayed, 0);
        Interlocked.Exchange(ref _totalDropped, 0);
        Interlocked.Exchange(ref _totalDuplicated, 0);
        Interlocked.Exchange(ref _totalReordered, 0);
        Interlocked.Exchange(ref _totalTampered, 0);
        Interlocked.Exchange(ref _totalBytes, 0);
        Interlocked.Exchange(ref _currentQueueDepth, 0);
        Interlocked.Exchange(ref _throttleQueueDepth, 0);
    }
}

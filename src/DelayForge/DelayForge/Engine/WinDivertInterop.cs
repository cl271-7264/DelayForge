using System.Runtime.InteropServices;

namespace DelayForge.Engine;

/// <summary>
/// P/Invoke declarations for WinDivert 2.2.
/// </summary>
internal static class WinDivertInterop
{
    private const string DllName = "WinDivert.dll";

    // WinDivert layer constants
    public const byte WINDIVERT_LAYER_NETWORK = 0;
    public const byte WINDIVERT_LAYER_FORWARD = 2;

    // WinDivert flags
    public const ulong WINDIVERT_FLAG_SNIFF = 0x0001;
    public const ulong WINDIVERT_FLAG_DROP = 0x0002;
    public const ulong WINDIVERT_FLAG_DEFAULT = 0x0000;

    // WinDivert address flags
    public const byte WINDIVERT_ADDRESS_FLAG_OUTBOUND = 0x01;
    public const byte WINDIVERT_ADDRESS_FLAG_LOOPBACK = 0x02;
    public const byte WINDIVERT_ADDRESS_FLAG_IMPOSTOR = 0x04;
    public const byte WINDIVERT_ADDRESS_FLAG_IPV6 = 0x08;

    // Helper checksum flags
    public const ulong WINDIVERT_HELPER_NO_ICMP_CHECKSUM = 0x0001;
    public const ulong WINDIVERT_HELPER_NO_ICMPV6_CHECKSUM = 0x0002;
    public const ulong WINDIVERT_HELPER_NO_TCP_CHECKSUM = 0x0004;
    public const ulong WINDIVERT_HELPER_NO_UDP_CHECKSUM = 0x0008;
    public const ulong WINDIVERT_HELPER_NO_IP_CHECKSUM = 0x0010;

    [StructLayout(LayoutKind.Sequential, Pack = 1)]
    public struct WINDIVERT_ADDRESS
    {
        public ulong Timestamp;
        public uint IfIdx;
        public uint SubIfIdx;
        public byte Network;
        public byte Protocol;
        public byte Flags;
        public byte Reserved;
    }

    [DllImport(DllName, CallingConvention = CallingConvention.Cdecl, SetLastError = true)]
    public static extern IntPtr WinDivertOpen(
        [MarshalAs(UnmanagedType.LPStr)] string filter,
        byte layer,
        short priority,
        ulong flags);

    [DllImport(DllName, CallingConvention = CallingConvention.Cdecl, SetLastError = true)]
    public static extern bool WinDivertRecv(
        IntPtr handle,
        byte[] pPacket,
        uint packetLen,
        out uint recvLen,
        ref WINDIVERT_ADDRESS pAddr);

    [DllImport(DllName, CallingConvention = CallingConvention.Cdecl, SetLastError = true)]
    public static extern bool WinDivertSend(
        IntPtr handle,
        byte[] pPacket,
        uint packetLen,
        out uint sendLen,
        ref WINDIVERT_ADDRESS pAddr);

    [DllImport(DllName, CallingConvention = CallingConvention.Cdecl, SetLastError = true)]
    public static extern bool WinDivertClose(IntPtr handle);

    [DllImport(DllName, CallingConvention = CallingConvention.Cdecl, SetLastError = true)]
    public static extern bool WinDivertHelperCalcChecksums(
        byte[] pPacket,
        uint packetLen,
        ref WINDIVERT_ADDRESS pAddr,
        ulong flags);

    [DllImport(DllName, CallingConvention = CallingConvention.Cdecl, SetLastError = true)]
    public static extern bool WinDivertHelperDecIPv4Address(
        [MarshalAs(UnmanagedType.LPStr)] string addrStr,
        ref uint pAddr);

    [DllImport(DllName, CallingConvention = CallingConvention.Cdecl, SetLastError = true)]
    public static extern bool WinDivertHelperIncIPv4Address(
        ref uint addr);

    [DllImport(DllName, CallingConvention = CallingConvention.Cdecl, SetLastError = true)]
    public static extern bool WinDivertHelperParseIPv4Address(
        [MarshalAs(UnmanagedType.LPStr)] string addrStr,
        ref uint pAddr);

    [DllImport(DllName, CallingConvention = CallingConvention.Cdecl, SetLastError = true)]
    public static extern bool WinDivertHelperParseIPv4Network(
        [MarshalAs(UnmanagedType.LPStr)] string addrStr,
        ref uint pAddr,
        ref uint pMask);

    [DllImport("kernel32.dll", SetLastError = true)]
    public static extern IntPtr LoadLibrary(string lpFileName);

    [DllImport("kernel32.dll", SetLastError = true)]
    public static extern bool SetDllDirectory(string lpPathName);
}

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

// WinDivert constants
const (
	WINDIVERT_LAYER_NETWORK = 0
	WINDIVERT_FLAG_DEFAULT  = 0
)

// WINDIVERT_ADDRESS (20 bytes, matching WinDivert 2.2 struct)
type WinDivertAddress struct {
	Timestamp uint64
	IfIdx     uint32
	SubIfIdx  uint32
	Network   uint8
	Protocol  uint8
	Flags     uint8
	Reserved  uint8
}

const (
	WINDIVERT_ADDRESS_FLAG_OUTBOUND = 0x01
	WINDIVERT_ADDRESS_FLAG_IPV6     = 0x08
)

var (
	hMod             uintptr
	_winDivertOpen   uintptr
	_winDivertRecv   uintptr
	_winDivertSend   uintptr
	_winDivertClose  uintptr
	_winDivertHelper uintptr
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLoadLibrary  = kernel32.NewProc("LoadLibraryW")
	procGetProcAddress = kernel32.NewProc("GetProcAddress")
)

func loadWinDivertDLL(path string) error {
	dllPath, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	ret, _, _ := procLoadLibrary.Call(uintptr(unsafe.Pointer(dllPath)))
	if ret == 0 {
		return fmt.Errorf("failed to load WinDivert.dll from %s", path)
	}
	hMod = ret

	_winDivertOpen = getProc("WinDivertOpen")
	_winDivertRecv = getProc("WinDivertRecv")
	_winDivertSend = getProc("WinDivertSend")
	_winDivertClose = getProc("WinDivertClose")
	_winDivertHelper = getProc("WinDivertHelperCalcChecksums")

	if _winDivertOpen == 0 || _winDivertRecv == 0 || _winDivertSend == 0 || _winDivertClose == 0 {
		return fmt.Errorf("failed to resolve WinDivert functions")
	}
	return nil
}

func getProc(name string) uintptr {
	ret, _, _ := procGetProcAddress.Call(hMod, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(name))))
	return ret
}

func winDivertOpen(filter string, layer byte, priority int16, flags uint64) (uintptr, error) {
	cFilter, err := syscall.BytePtrFromString(filter)
	if err != nil {
		return 0, err
	}
	ret, _, err := syscall.SyscallN(_winDivertOpen,
		uintptr(unsafe.Pointer(cFilter)),
		uintptr(layer),
		uintptr(priority),
		uintptr(flags),
	)
	if ret == 0 || ret == ^uintptr(0) {
		return 0, fmt.Errorf("WinDivertOpen failed: %v", err)
	}
	return ret, nil
}

func winDivertRecv(handle uintptr, buf []byte, addr *WinDivertAddress) (int, error) {
	var recvLen uint32
	ret, _, err := syscall.SyscallN(_winDivertRecv,
		handle,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&recvLen)),
		uintptr(unsafe.Pointer(addr)),
	)
	if ret == 0 {
		return 0, fmt.Errorf("WinDivertRecv failed: %v", err)
	}
	return int(recvLen), nil
}

func winDivertSend(handle uintptr, buf []byte, addr *WinDivertAddress) (int, error) {
	var sentLen uint32
	ret, _, err := syscall.SyscallN(_winDivertSend,
		handle,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&sentLen)),
		uintptr(unsafe.Pointer(addr)),
	)
	if ret == 0 {
		return 0, fmt.Errorf("WinDivertSend failed: %v", err)
	}
	return int(sentLen), nil
}

func winDivertClose(handle uintptr) {
	syscall.SyscallN(_winDivertClose, handle)
}

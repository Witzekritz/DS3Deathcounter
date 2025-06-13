package memory

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
    PROCESS_VM_READ           = 0x0010
    PROCESS_QUERY_INFORMATION = 0x0400
)

var (
    kernel32              = windows.NewLazyDLL("kernel32.dll")
    procReadProcessMemory = kernel32.NewProc("ReadProcessMemory")
    procIsWow64Process    = kernel32.NewProc("IsWow64Process")
)

// ReadProcessMemory reads data from a specified process memory address
func ReadProcessMemory(process windows.Handle, baseAddress uintptr, size uint) ([]byte, error) {
    var read uintptr
    buf := make([]byte, size)
    ret, _, err := procReadProcessMemory.Call(
        uintptr(process),
        baseAddress,
        uintptr(unsafe.Pointer(&buf[0])),
        uintptr(size),
        uintptr(unsafe.Pointer(&read)),
    )
    if ret == 0 {
        return nil, fmt.Errorf("ReadProcessMemory failed: %v", err)
    }
    if read != uintptr(size) {
        return nil, fmt.Errorf("partial read: got %d bytes, expected %d", read, size)
    }
    return buf, nil
}

// IsWow64Process determines if a process is 32-bit (running under WOW64)
func IsWow64Process(process windows.Handle) (bool, error) {
    var isWow64 bool
    ret, _, err := procIsWow64Process.Call(
        uintptr(process),
        uintptr(unsafe.Pointer(&isWow64)),
    )
    if ret == 0 {
        return false, fmt.Errorf("IsWow64Process failed: %v", err)
    }
    return isWow64, nil
}

// GetModuleBaseAddress gets the base address of a module in a process
func GetModuleBaseAddress(processID uint32, moduleName string) (uintptr, error) {
    snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPMODULE|windows.TH32CS_SNAPMODULE32, processID)
    if err != nil {
        return 0, err
    }
    defer windows.CloseHandle(snapshot)

    var me windows.ModuleEntry32
    me.Size = uint32(unsafe.Sizeof(me))

    err = windows.Module32First(snapshot, &me)
    if err != nil {
        return 0, err
    }

    for {
        if windows.UTF16ToString(me.Module[:]) == moduleName {
            return uintptr(me.ModBaseAddr), nil
        }
        err = windows.Module32Next(snapshot, &me)
        if err != nil {
            if err == syscall.ERROR_NO_MORE_FILES {
                return 0, fmt.Errorf("module %s not found", moduleName)
            }
            return 0, err
        }
    }
}
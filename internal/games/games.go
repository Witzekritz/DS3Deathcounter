package games

import (
	"fmt"
	"log"
	"strings"
	"syscall"
	"unsafe"

	"dsdeathcounter/internal/memory"

	"golang.org/x/sys/windows"
)

// Game defines memory locations for each supported game
type Game struct {
    Name        string
    ProcessName string
    Offsets32   []int64
    Offsets64   []int64
}

// Process holds information about a found game process
type Process struct {
    Handle    windows.Handle
    ProcessID uint32
    Game      *Game
}

// GetSupportedGames returns all supported games with memory offsets
func GetSupportedGames() []Game {
    return []Game{
        {
            Name:        "Dark Souls",
            ProcessName: "DARKSOULS",
            Offsets32:   []int64{0xF78700, 0x5C},
            Offsets64:   nil,
        },
        {
            Name:        "Dark Souls II",
            ProcessName: "DarkSoulsII",
            Offsets32:   []int64{0x1150414, 0x74, 0xB8, 0x34, 0x4, 0x28C, 0x100},
            Offsets64:   []int64{0x16148F0, 0xD0, 0x490, 0x104},
        },
        {
            Name:        "Dark Souls III",
            ProcessName: "DarkSoulsIII",
            Offsets32:   nil,
            Offsets64:   []int64{0x47572B8, 0x98},
        },
        {
            Name:        "Dark Souls Remastered",
            ProcessName: "DarkSoulsRemastered",
            Offsets32:   nil,
            Offsets64:   []int64{0x1C8A530, 0x98},
        },
        {
            Name:        "Sekiro",
            ProcessName: "sekiro",
            Offsets32:   nil,
            Offsets64:   []int64{0x3D5AAC0, 0x90},
        },
    }
}

// FindGameProcess locates a supported game process
func FindGameProcess() (Process, error) {
    snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
    if err != nil {
        return Process{}, err
    }
    defer windows.CloseHandle(snapshot)

    var pe windows.ProcessEntry32
    pe.Size = uint32(unsafe.Sizeof(pe))

    err = windows.Process32First(snapshot, &pe)
    if err != nil {
        return Process{}, err
    }

    for {
        processName := windows.UTF16ToString(pe.ExeFile[:])
        processName = strings.TrimSuffix(processName, ".exe")

        for i := range GetSupportedGames() {
            game := &GetSupportedGames()[i]
            if processName == game.ProcessName {
                process, err := windows.OpenProcess(memory.PROCESS_VM_READ|memory.PROCESS_QUERY_INFORMATION, false, pe.ProcessID)
                if err != nil {
                    log.Printf("Found %s but could not open process: %v", game.Name, err)
                    continue
                }
                return Process{
                    Handle:    process,
                    ProcessID: pe.ProcessID,
                    Game:      game,
                }, nil
            }
        }

        err = windows.Process32Next(snapshot, &pe)
        if err != nil {
            if err == syscall.ERROR_NO_MORE_FILES {
                return Process{}, fmt.Errorf("no supported game process found")
            }
            return Process{}, err
        }
    }
}

// GetDeathCount reads the death count from a game process
func GetDeathCount(process Process) (int32, error) {
    // Check if process is 32-bit or 64-bit
    isWow64, err := memory.IsWow64Process(process.Handle)
    if err != nil {
        return 0, fmt.Errorf("failed to determine process architecture: %v", err)
    }

    // Get appropriate offsets for architecture
    var offsets []int64
    if isWow64 {
        if process.Game.Offsets32 == nil {
            return 0, fmt.Errorf("%s doesn't support 32-bit version", process.Game.Name)
        }
        offsets = process.Game.Offsets32
    } else {
        if process.Game.Offsets64 == nil {
            return 0, fmt.Errorf("%s doesn't support 64-bit version", process.Game.Name)
        }
        offsets = process.Game.Offsets64
    }

    // Get the base address of the module
    baseAddr, err := memory.GetModuleBaseAddress(process.ProcessID, process.Game.ProcessName+".exe")
    if err != nil {
        return 0, fmt.Errorf("failed to get module base address: %v", err)
    }

    // Follow pointer chain
    address := int64(baseAddr)
    for i, offset := range offsets {
        address += offset

        if i < len(offsets)-1 {
            // Read pointer
            pointerData, err := memory.ReadProcessMemory(process.Handle, uintptr(address), 8)
            if err != nil {
                return 0, fmt.Errorf("failed to read pointer at offset %d: %v", i, err)
            }

            // Interpret based on architecture
            if isWow64 {
                address = int64(*(*int32)(unsafe.Pointer(&pointerData[0])))
            } else {
                address = *(*int64)(unsafe.Pointer(&pointerData[0]))
            }

            if address == 0 {
                return 0, fmt.Errorf("null pointer encountered at offset %d", i)
            }
        }
    }

    // Read final value (death count)
    deathData, err := memory.ReadProcessMemory(process.Handle, uintptr(address), 4)
    if err != nil {
        return 0, fmt.Errorf("failed to read death count: %v", err)
    }

    return *(*int32)(unsafe.Pointer(&deathData[0])), nil
}
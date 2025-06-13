package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
    PROCESS_VM_READ           = 0x0010
    PROCESS_QUERY_INFORMATION = 0x0400
)

// Game defines memory locations for each supported game
type Game struct {
    Name      string
    ProcessName string
    Offsets32 []int64
    Offsets64 []int64
}

// All supported games with their memory offsets
var supportedGames = []Game{
    {
        Name:      "Dark Souls",
        ProcessName: "DARKSOULS",
        Offsets32: []int64{0xF78700, 0x5C},
        Offsets64: nil,
    },
    {
        Name:      "Dark Souls II",
        ProcessName: "DarkSoulsII",
        Offsets32: []int64{0x1150414, 0x74, 0xB8, 0x34, 0x4, 0x28C, 0x100},
        Offsets64: []int64{0x16148F0, 0xD0, 0x490, 0x104},
    },
    {
        Name:      "Dark Souls III",
        ProcessName: "DarkSoulsIII",
        Offsets32: nil,
        Offsets64: []int64{0x47572B8, 0x98},
    },
    {
        Name:      "Dark Souls Remastered",
        ProcessName: "DarkSoulsRemastered",
        Offsets32: nil,
        Offsets64: []int64{0x1C8A530, 0x98},
    },
    {
        Name:      "sekiro",
        ProcessName: "sekiro",
        Offsets32: nil,
        Offsets64: []int64{0x3D5AAC0, 0x90},
    },
}

var (
    kernel32              = windows.NewLazyDLL("kernel32.dll")
    procReadProcessMemory = kernel32.NewProc("ReadProcessMemory")
    procIsWow64Process    = kernel32.NewProc("IsWow64Process")
)

func readProcessMemory(process windows.Handle, baseAddress uintptr, size uint) ([]byte, error) {
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

func isWow64Process(process windows.Handle) (bool, error) {
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

func getModuleBaseAddress(processID uint32, moduleName string) (uintptr, error) {
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

func findGameProcess() (windows.Handle, uint32, *Game, error) {
    snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
    if err != nil {
        return 0, 0, nil, err
    }
    defer windows.CloseHandle(snapshot)

    var pe windows.ProcessEntry32
    pe.Size = uint32(unsafe.Sizeof(pe))

    err = windows.Process32First(snapshot, &pe)
    if err != nil {
        return 0, 0, nil, err
    }

    for {
        processName := windows.UTF16ToString(pe.ExeFile[:])
        processName = strings.TrimSuffix(processName, ".exe")
        
        for i := range supportedGames {
            if processName == supportedGames[i].ProcessName {
                process, err := windows.OpenProcess(PROCESS_VM_READ|PROCESS_QUERY_INFORMATION, false, pe.ProcessID)
                if err != nil {
                    log.Printf("Found %s but could not open process: %v", supportedGames[i].Name, err)
                    continue
                }
                return process, pe.ProcessID, &supportedGames[i], nil
            }
        }

        err = windows.Process32Next(snapshot, &pe)
        if err != nil {
            if err == syscall.ERROR_NO_MORE_FILES {
                return 0, 0, nil, fmt.Errorf("no supported game process found")
            }
            return 0, 0, nil, err
        }
    }
}

func getDeathCount(process windows.Handle, processID uint32, game *Game) (int32, error) {
    // Check if process is 32-bit or 64-bit
    isWow64, err := isWow64Process(process)
    if err != nil {
        return 0, fmt.Errorf("failed to determine process architecture: %v", err)
    }

    // Get appropriate offsets for architecture
    var offsets []int64
    if isWow64 {
        if game.Offsets32 == nil {
            return 0, fmt.Errorf("%s doesn't support 32-bit version", game.Name)
        }
        offsets = game.Offsets32
    } else {
        if game.Offsets64 == nil {
            return 0, fmt.Errorf("%s doesn't support 64-bit version", game.Name)
        }
        offsets = game.Offsets64
    }

    // Get the base address of the module
    baseAddr, err := getModuleBaseAddress(processID, game.ProcessName+".exe")
    if err != nil {
        return 0, fmt.Errorf("failed to get module base address: %v", err)
    }

    // Follow pointer chain
    address := int64(baseAddr)
    for i, offset := range offsets {
        address += offset
        
        if i < len(offsets)-1 {
            // Read pointer
            pointerData, err := readProcessMemory(process, uintptr(address), 8)
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
    deathData, err := readProcessMemory(process, uintptr(address), 4)
    if err != nil {
        return 0, fmt.Errorf("failed to read death count: %v", err)
    }

    return *(*int32)(unsafe.Pointer(&deathData[0])), nil
}

func main() {
    var currentDeaths int32
    var currentGameName string = "No game detected"

    // Start update goroutine
    go func() {
        for {
            process, processID, game, err := findGameProcess()
            if err != nil {
                if currentGameName != "No game detected" {
                    log.Printf("Game process closed or not found: %v", err)
                    currentGameName = "No game detected"
                }
                time.Sleep(2 * time.Second)
                continue
            }

            if currentGameName != game.Name {
                currentGameName = game.Name
                log.Printf("Found game: %s", currentGameName)
            }

            deaths, err := getDeathCount(process, processID, game)
            if err != nil {
                log.Printf("Error reading death count: %v", err)
                windows.CloseHandle(process)
                time.Sleep(2 * time.Second)
                continue
            }

            if deaths != currentDeaths {
                currentDeaths = deaths
                log.Printf("%s deaths updated: %d", currentGameName, currentDeaths)
            }

            windows.CloseHandle(process)
            time.Sleep(500 * time.Millisecond)
        }
    }()

	// Web server handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
		@font-face {
    	font-family: "Garamond Bold";
    	src: url("https://db.onlinewebfonts.com/t/4a7a8bacf63ac3e32b8cd692363de2a9.eot");
    	src: url("https://db.onlinewebfonts.com/t/4a7a8bacf63ac3e32b8cd692363de2a9.eot?#iefix")format("embedded-opentype"),
    	url("https://db.onlinewebfonts.com/t/4a7a8bacf63ac3e32b8cd692363de2a9.woff2")format("woff2"),
    	url("https://db.onlinewebfonts.com/t/4a7a8bacf63ac3e32b8cd692363de2a9.woff")format("woff"),
    	url("https://db.onlinewebfonts.com/t/4a7a8bacf63ac3e32b8cd692363de2a9.ttf")format("truetype"),
    	url("https://db.onlinewebfonts.com/t/4a7a8bacf63ac3e32b8cd692363de2a9.svg#Garamond Bold")format("svg");
}
        body {
            background: transparent;
            color: white;
            font-family: "Garamond Bold";
            font-size: 24px;
            margin: 0;
            text-shadow: 2px 2px 2px #000;
            display: flex;
            align-items: center;
        }
        #counter {
            display: flex;
            align-items: center;
            gap: 10px;
        }
        img {
            height: 3em;
            width: auto;
            vertical-align: middle;
            mix-blend-mode: screen;
			transform: scaleX(-1);
        }
    </style>
</head>
<body>
    <div id="counter">
        <a href="https://imgbb.com/"><img src="https://i.ibb.co/q3hkbryw/skull.png" alt="skull" border="0"></a>
        DEATHS: <span id="deaths">%d</span>
    </div>
    <script>
        function updateDeaths() {
            fetch(window.location.href)
                .then(response => response.text())
                .then(html => {
                    const parser = new DOMParser();
                    const doc = parser.parseFromString(html, 'text/html');
                    const newDeaths = doc.getElementById('deaths').textContent;
                    document.getElementById('deaths').textContent = newDeaths;
                });
        }
        setInterval(updateDeaths, 1000);
    </script>
</body>
</html>`, currentDeaths)
	})

	log.Println("Starting web server on http://localhost:8080")
	if err := http.ListenAndServe("127.0.0.1:8080", nil); err != nil {
		log.Fatal(err)
	}
}
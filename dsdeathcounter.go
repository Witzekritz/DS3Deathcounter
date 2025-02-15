package main

import (
	"fmt"
	"log"
	"net/http"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	PROCESS_VM_READ           = 0x0010
	PROCESS_QUERY_INFORMATION = 0x0400
	DS3_BASE_ADDR            = 0x47572B8
	DS3_OFFSET               = 0x98
)

var (
	kernel32              = windows.NewLazyDLL("kernel32.dll")
	procReadProcessMemory = kernel32.NewProc("ReadProcessMemory")
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

func findDS3Process() (windows.Handle, uint32, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, 0, err
	}
	defer windows.CloseHandle(snapshot)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))

	err = windows.Process32First(snapshot, &pe)
	if err != nil {
		return 0, 0, err
	}

	for {
		if windows.UTF16ToString(pe.ExeFile[:]) == "DarkSoulsIII.exe" {
			process, err := windows.OpenProcess(PROCESS_VM_READ|PROCESS_QUERY_INFORMATION, false, pe.ProcessID)
			if err != nil {
				return 0, 0, err
			}
			return process, pe.ProcessID, nil
		}

		err = windows.Process32Next(snapshot, &pe)
		if err != nil {
			if err == syscall.ERROR_NO_MORE_FILES {
				return 0, 0, fmt.Errorf("Dark Souls III process not found")
			}
			return 0, 0, err
		}
	}
}

func getDeathCount(process windows.Handle, processID uint32) (int32, error) {
	// Get the base address of the module
	baseAddr, err := getModuleBaseAddress(processID, "DarkSoulsIII.exe")
	if err != nil {
		return 0, fmt.Errorf("failed to get module base address: %v", err)
	}

	// Calculate the absolute address
	targetAddr := baseAddr + uintptr(DS3_BASE_ADDR)
	
	//log.Printf("Reading from base address: 0x%X", targetAddr)

	// Read pointer at base address
	baseData, err := readProcessMemory(process, targetAddr, 8)
	if err != nil {
		return 0, fmt.Errorf("failed to read base pointer: %v", err)
	}

	// Get pointer from base address
	basePointer := *(*int64)(unsafe.Pointer(&baseData[0]))
	if basePointer == 0 {
		return 0, fmt.Errorf("null pointer read from base address")
	}

	//log.Printf("Read pointer: 0x%X", basePointer)
	
	// Read final value using offset
	finalAddr := uintptr(basePointer + DS3_OFFSET)
	//log.Printf("Reading final value from: 0x%X", finalAddr)

	deathData, err := readProcessMemory(process, finalAddr, 4)
	if err != nil {
		return 0, fmt.Errorf("failed to read death count: %v", err)
	}

	return *(*int32)(unsafe.Pointer(&deathData[0])), nil
}

func main() {
	var currentDeaths int32

	// Start update goroutine
	go func() {
		for {
			process, processID, err := findDS3Process()
			if err != nil {
				log.Printf("Error finding DS3 process: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			deaths, err := getDeathCount(process, processID)
			if err != nil {
				log.Printf("Error reading death count: %v", err)
				windows.CloseHandle(process)
				time.Sleep(5 * time.Second)
				continue
			}

			if deaths != currentDeaths {
				currentDeaths = deaths
				log.Printf("Deaths updated: %d", currentDeaths)
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
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
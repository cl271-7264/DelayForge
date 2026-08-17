package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

//go:embed webui/index.html
var webuiFS embed.FS

var (
	engine  *DamageEngine
	dllPath string
	sysPath string
	logFile *os.File
)

func main() {
	// Panic recovery — write to file so user can see what went wrong
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("PANIC: %v\n", r)
			if logFile != nil {
				logFile.WriteString(msg)
				logFile.Close()
			}
		}
	}()

	// Log to file instead of stdout (WinExe has no console)
	var err error
	logFile, err = os.Create("delayforge.log")
	if err == nil {
		log.SetOutput(logFile)
	} else {
		log.SetOutput(os.Stderr)
	}
	log.SetFlags(log.Ltime)

	log.Println("=== DelayForge starting ===")

	// Find WinDivert DLL and SYS next to executable
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	log.Printf("Exe dir: %s", exeDir)

	// Try local directory first, then current working directory
	paths := []string{
		exeDir,
		".",
		filepath.Join(exeDir, "windivert"),
	}

	for _, dir := range paths {
		dll := filepath.Join(dir, "WinDivert.dll")
		sys := filepath.Join(dir, "WinDivert64.sys")
		if fileExists(dll) && fileExists(sys) {
			dllPath = dll
			sysPath = sys
			log.Printf("Found WinDivert in: %s", dir)
			break
		}
	}

	if dllPath == "" {
		// Extract from embedded DLL (we only embed the DLL, user provides SYS)
		// Actually try to find SYS from NuGet cache or temp
		log.Println("WinDivert not found locally, trying embedded extraction...")
		extractDir := filepath.Join(os.TempDir(), "delayforge")
		os.MkdirAll(extractDir, 0755)

		// Try to find in NuGet cache
		nugetPaths := []string{
			filepath.Join(os.Getenv("USERPROFILE"), ".nuget", "packages", "native.windivert", "2.2.2", "runtimes", "win-x64", "native"),
		}
		for _, dir := range nugetPaths {
			dll := filepath.Join(dir, "WinDivert.dll")
			sys := filepath.Join(dir, "WinDivert64.sys")
			if fileExists(dll) && fileExists(sys) {
				dllPath = dll
				sysPath = sys
				log.Printf("Found WinDivert in NuGet cache: %s", dir)
				break
			}
		}
	}

	if dllPath == "" {
		log.Println("ERROR: WinDivert.dll and WinDivert64.sys not found!")
		log.Println("Place them next to the exe, or in a 'windivert' subfolder.")
		// Show message box for WinExe
		showMessageBox("DelayForge", "WinDivert.dll and WinDivert64.sys not found.\n\nPlace them next to the exe.")
		return
	}

	// Load WinDivert DLL
	log.Println("Loading WinDivert DLL...")
	if err := loadWinDivertDLL(dllPath); err != nil {
		log.Printf("ERROR loading WinDivert: %v", err)
		showMessageBox("DelayForge", fmt.Sprintf("Failed to load WinDivert:\n%v", err))
		return
	}
	log.Println("WinDivert loaded OK")

	engine = NewDamageEngine()

	// Setup HTTP server
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/start", handleStart)
	http.HandleFunc("/api/stop", handleStop)
	http.HandleFunc("/api/config", handleConfig)
	http.HandleFunc("/api/stats", handleStats)
	http.HandleFunc("/api/filter", handleFilterPreview)

	port := 8380
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	log.Printf("Starting HTTP server on %s", addr)

	// Open browser automatically
	go func() {
		time.Sleep(800 * time.Millisecond)
		openBrowser(fmt.Sprintf("http://%s", addr))
	}()

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Printf("HTTP server error: %v", err)
		showMessageBox("DelayForge", fmt.Sprintf("HTTP server error:\n%v", err))
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, _ := webuiFS.ReadFile("webui/index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", 405)
		return
	}

	var cfg DamageConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	filter := BuildFilter(cfg)
	log.Printf("Starting with filter: %s", filter)

	handle, err := winDivertOpen(filter, WINDIVERT_LAYER_NETWORK, 0, WINDIVERT_FLAG_DEFAULT)
	if err != nil {
		msg := fmt.Sprintf("WinDivertOpen failed: %v\n\nAre you running as Administrator?", err)
		log.Println(msg)
		http.Error(w, msg, 500)
		return
	}

	engine.Start(handle, cfg)
	log.Println("Engine started")
	json.NewEncoder(w).Encode(map[string]string{"status": "running"})
}

func handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", 405)
		return
	}
	engine.Stop()
	if engine.handle != 0 {
		winDivertClose(engine.handle)
		engine.handle = 0
	}
	log.Println("Engine stopped")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", 405)
		return
	}
	var cfg DamageConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	engine.UpdateConfig(cfg)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"processed":    engine.stats.Processed.Load(),
		"bytes":        engine.stats.Bytes.Load(),
		"delayed":      engine.stats.Delayed.Load(),
		"dropped":      engine.stats.Dropped.Load(),
		"duplicated":   engine.stats.Duplicated.Load(),
		"reordered":    engine.stats.Reordered.Load(),
		"tampered":     engine.stats.Tampered.Load(),
		"delayQueue":   engine.delayQueue.Len(),
		"reorderQueue": engine.reorderQueue.Len(),
		"throttleQueue": engine.throttleQueue.Len(),
		"running":      engine.running,
	})
}

func handleFilterPreview(w http.ResponseWriter, r *http.Request) {
	var cfg DamageConfig
	json.NewDecoder(r.Body).Decode(&cfg)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"filter": BuildFilter(cfg)})
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}
	c := exec.Command(cmd, args...)
	c.Start()
}

func showMessageBox(title, msg string) {
	// Use PowerShell to show a message box for WinExe apps
	ps := fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.MessageBox]::Show('%s', '%s', 'OK', 'Error')`, msg, title)
	c := exec.Command("powershell", "-Command", ps)
	c.Run()
}

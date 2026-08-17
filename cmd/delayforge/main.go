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

//go:embed webui/*
var webuiFS embed.FS

var (
	engine   *DamageEngine
	dllPath  string
	sysPath  string
)

func main() {
	log.SetFlags(0)
	log.SetOutput(os.Stdout)

	// Find WinDivert DLL and SYS next to executable or in embedded resources
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

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
			break
		}
	}

	if dllPath == "" {
		// Try to extract from embedded resources (for single-file distribution)
		extractDir := filepath.Join(os.TempDir(), "delayforge")
		os.MkdirAll(extractDir, 0755)
		extractFile(webuiFS, "windivert/WinDivert.dll", filepath.Join(extractDir, "WinDivert.dll"))
		extractFile(webuiFS, "windivert/WinDivert64.sys", filepath.Join(extractDir, "WinDivert64.sys"))
		if fileExists(filepath.Join(extractDir, "WinDivert.dll")) {
			dllPath = filepath.Join(extractDir, "WinDivert.dll")
			sysPath = filepath.Join(extractDir, "WinDivert64.sys")
		}
	}

	if dllPath == "" {
		log.Fatal("ERROR: WinDivert.dll not found. Place it next to the executable.")
	}

	log.Printf("WinDivert DLL: %s", dllPath)
	log.Printf("WinDivert SYS: %s", sysPath)

	// Load WinDivert DLL
	if err := loadWinDivertDLL(dllPath); err != nil {
		log.Fatalf("ERROR: Failed to load WinDivert: %v", err)
	}
	log.Println("WinDivert loaded successfully.")

	engine = NewDamageEngine()

	// Setup HTTP server
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/start", handleStart)
	http.HandleFunc("/api/stop", handleStop)
	http.HandleFunc("/api/config", handleConfig)
	http.HandleFunc("/api/stats", handleStats)
	http.HandleFunc("/api/filter", handleFilterPreview)

	port := 8380
	if p := os.Getenv("PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}

	addr := fmt.Sprintf("http://127.0.0.1:%d", port)
	log.Printf("DelayForge UI: %s", addr)
	log.Println("Press Ctrl+C to stop.")

	// Open browser automatically
	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser(addr)
	}()

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("HTTP server error: %v", err)
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
		http.Error(w, fmt.Sprintf("WinDivertOpen failed: %v\n\nAre you running as Administrator?", err), 500)
		return
	}

	engine.Start(handle, cfg)

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
		"processed":   engine.stats.Processed.Load(),
		"bytes":       engine.stats.Bytes.Load(),
		"delayed":     engine.stats.Delayed.Load(),
		"dropped":     engine.stats.Dropped.Load(),
		"duplicated":  engine.stats.Duplicated.Load(),
		"reordered":   engine.stats.Reordered.Load(),
		"tampered":    engine.stats.Tampered.Load(),
		"delayQueue":  engine.delayQueue.Len(),
		"reorderQueue": engine.reorderQueue.Len(),
		"throttleQueue": engine.throttleQueue.Len(),
		"running":     engine.running,
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

func extractFile(fs embed.FS, embedPath, destPath string) {
	data, err := fs.ReadFile(embedPath)
	if err != nil {
		return
	}
	os.WriteFile(destPath, data, 0755)
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

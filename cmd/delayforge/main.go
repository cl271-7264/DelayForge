package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

var (
	engine  *DamageEngine
	dllPath string
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC: %v", r)
		}
	}()

	logFile, _ := os.Create("delayforge.log")
	if logFile != nil {
		log.SetOutput(logFile)
	}
	log.SetFlags(log.Ltime)
	log.Println("=== DelayForge starting ===")

	// Find WinDivert
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	searchPaths := []string{
		exeDir, ".",
		filepath.Join(exeDir, "windivert"),
		filepath.Join(os.Getenv("USERPROFILE"), ".nuget", "packages", "native.windivert", "2.2.2", "runtimes", "win-x64", "native"),
	}
	for _, dir := range searchPaths {
		dll := filepath.Join(dir, "WinDivert.dll")
		sys := filepath.Join(dir, "WinDivert64.sys")
		if fileExists(dll) && fileExists(sys) {
			dllPath = dll
			log.Printf("Found WinDivert: %s", dir)
			break
		}
	}
	if dllPath == "" {
		walk.MsgBox(nil, "Error", "WinDivert.dll not found.\nPlace it next to the exe.", walk.MsgBoxIconError)
		return
	}
	if err := loadWinDivertDLL(dllPath); err != nil {
		walk.MsgBox(nil, "Error", fmt.Sprintf("Load WinDivert failed:\n%v", err), walk.MsgBoxIconError)
		return
	}
	log.Println("WinDivert loaded OK")

	engine = NewDamageEngine()

	// --- Widget refs ---
	var mainWindow *walk.MainWindow
	var btnToggle *walk.PushButton
	var lblProcessed, lblBytes, lblDelayed, lblDropped *walk.Label
	var lblDuplicated, lblReordered, lblTampered *walk.Label
	var cmbDirection, cmbProtocol *walk.ComboBox
	var leIP, lePort *walk.LineEdit
	var slLatency, slJitter, slLoss, slDup, slReorder, slTamper, slThrottle *walk.Slider

	err := MainWindow{
		AssignTo: &mainWindow,
		Title:    "DelayForge",
		MinSize:  Size{Width: 460, Height: 600},
		Size:     Size{Width: 480, Height: 660},
		Layout:   VBox{},
		Children: []Widget{
			TabWidget{
				Pages: []TabPage{
					{
						Title:  "Damage",
						Layout: VBox{Margins: Margins{Top: 8, Left: 12, Right: 12, Bottom: 8}},
						Children: []Widget{
							GroupBox{Title: "⏱ Latency", Layout: Grid{Columns: 3},
								Children: []Widget{
									Label{Text: "Delay:"},
									Slider{AssignTo: &slLatency, MinValue: 0, MaxValue: 3000},
									Label{Text: "ms"},
									Label{Text: "Jitter:"},
									Slider{AssignTo: &slJitter, MinValue: 0, MaxValue: 500},
									Label{Text: "±ms"},
								}},
							GroupBox{Title: "✕ Packet Loss", Layout: Grid{Columns: 3},
								Children: []Widget{
									Label{Text: "Rate:"},
									Slider{AssignTo: &slLoss, MinValue: 0, MaxValue: 100},
									Label{Text: "%"},
								}},
							GroupBox{Title: "⧉ Duplicate", Layout: Grid{Columns: 3},
								Children: []Widget{
									Label{Text: "Rate:"},
									Slider{AssignTo: &slDup, MinValue: 0, MaxValue: 50},
									Label{Text: "%"},
								}},
							GroupBox{Title: "⇅ Reorder", Layout: Grid{Columns: 3},
								Children: []Widget{
									Label{Text: "Rate:"},
									Slider{AssignTo: &slReorder, MinValue: 0, MaxValue: 50},
									Label{Text: "%"},
								}},
							GroupBox{Title: "⚡ Tamper", Layout: Grid{Columns: 3},
								Children: []Widget{
									Label{Text: "Rate:"},
									Slider{AssignTo: &slTamper, MinValue: 0, MaxValue: 50},
									Label{Text: "%"},
								}},
							GroupBox{Title: "🔒 Throttle", Layout: Grid{Columns: 3},
								Children: []Widget{
									Label{Text: "Max:"},
									Slider{AssignTo: &slThrottle, MinValue: 0, MaxValue: 10000},
									Label{Text: "kbps"},
								}},
						},
					},
					{
						Title:  "Filters",
						Layout: VBox{Margins: Margins{Top: 8, Left: 12, Right: 12, Bottom: 8}},
						Children: []Widget{
							GroupBox{Title: "Filter Rules", Layout: Grid{Columns: 2, Spacing: 6},
								Children: []Widget{
									Label{Text: "Direction:"},
									ComboBox{AssignTo: &cmbDirection, Model: []string{"Both", "Outbound", "Inbound"}, CurrentIndex: 0},
									Label{Text: "Protocol:"},
									ComboBox{AssignTo: &cmbProtocol, Model: []string{"All", "TCP", "UDP", "ICMP"}, CurrentIndex: 0},
									Label{Text: "IP / CIDR:"},
									LineEdit{AssignTo: &leIP},
									Label{Text: "Ports:"},
									LineEdit{AssignTo: &lePort},
								}},
						},
					},
					{
						Title:  "Stats",
						Layout: VBox{Margins: Margins{Top: 8, Left: 12, Right: 12, Bottom: 8}},
						Children: []Widget{
							GroupBox{Title: "Live Statistics", Layout: Grid{Columns: 2, Spacing: 4},
								Children: []Widget{
									Label{Text: "Processed:"}, Label{AssignTo: &lblProcessed, Text: "0"},
									Label{Text: "Bytes:"}, Label{AssignTo: &lblBytes, Text: "0 B"},
									Label{Text: "Delayed:"}, Label{AssignTo: &lblDelayed, Text: "0"},
									Label{Text: "Dropped:"}, Label{AssignTo: &lblDropped, Text: "0"},
									Label{Text: "Duplicated:"}, Label{AssignTo: &lblDuplicated, Text: "0"},
									Label{Text: "Reordered:"}, Label{AssignTo: &lblReordered, Text: "0"},
									Label{Text: "Tampered:"}, Label{AssignTo: &lblTampered, Text: "0"},
								}},
						},
					},
				},
			},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					PushButton{
						AssignTo: &btnToggle,
						Text:     "▶ Start",
						OnClicked: func() {
							if engine.running {
								engine.Stop()
								if engine.handle != 0 {
									winDivertClose(engine.handle)
									engine.handle = 0
								}
								btnToggle.SetText("▶ Start")
								log.Println("Stopped")
							} else {
								cfg := getCfg(cmbDirection, cmbProtocol, leIP, lePort,
									slLatency, slJitter, slLoss, slDup, slReorder, slTamper, slThrottle)
								filter := BuildFilter(cfg)
								log.Printf("Filter: %s", filter)
								handle, err := winDivertOpen(filter, WINDIVERT_LAYER_NETWORK, 0, WINDIVERT_FLAG_DEFAULT)
								if err != nil {
									walk.MsgBox(mainWindow, "Error",
										fmt.Sprintf("WinDivertOpen failed:\n%v\n\nRun as Administrator!", err),
										walk.MsgBoxIconError)
									return
								}
								engine.Start(handle, cfg)
								btnToggle.SetText("⏹ Stop")
								log.Println("Started")
							}
						},
					},
					HSpacer{},
				},
			},
		},
	}.Create()

	if err != nil {
		log.Printf("UI error: %v", err)
		return
	}

	// Stats refresh
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		for range ticker.C {
			if mainWindow == nil || mainWindow.IsDisposed() {
				return
			}
			mainWindow.Synchronize(func() {
				lblProcessed.SetText(strconv.FormatInt(engine.stats.Processed.Load(), 1))
				lblBytes.SetText(fmtBytes(engine.stats.Bytes.Load()))
				lblDelayed.SetText(strconv.FormatInt(engine.stats.Delayed.Load(), 1))
				lblDropped.SetText(strconv.FormatInt(engine.stats.Dropped.Load(), 1))
				lblDuplicated.SetText(strconv.FormatInt(engine.stats.Duplicated.Load(), 1))
				lblReordered.SetText(strconv.FormatInt(engine.stats.Reordered.Load(), 1))
				lblTampered.SetText(strconv.FormatInt(engine.stats.Tampered.Load(), 1))
			})
			if engine.running {
				engine.UpdateConfig(getCfg(cmbDirection, cmbProtocol, leIP, lePort,
					slLatency, slJitter, slLoss, slDup, slReorder, slTamper, slThrottle))
			}
		}
	}()

	mainWindow.Run()
}

func getCfg(cmbDir, cmbProto *walk.ComboBox, leIP, lePort *walk.LineEdit,
	slLat, slJit, slLoss, slDup, slReorder, slTamper, slThrottle *walk.Slider) DamageConfig {
	dir := "both"
	if cmbDir != nil {
		switch cmbDir.CurrentIndex() {
		case 1:
			dir = "outbound"
		case 2:
			dir = "inbound"
		}
	}
	proto := "any"
	if cmbProto != nil {
		switch cmbProto.CurrentIndex() {
		case 1:
			proto = "tcp"
		case 2:
			proto = "udp"
		case 3:
			proto = "icmp"
		}
	}
	port, ip := "", ""
	if lePort != nil {
		port = lePort.Text()
	}
	if leIP != nil {
		ip = leIP.Text()
	}
	return DamageConfig{
		LatencyMs:     slLat.Value(),
		JitterMs:      slJit.Value(),
		PacketLossPct: float64(slLoss.Value()),
		DuplicatePct:  float64(slDup.Value()),
		ReorderPct:    float64(slReorder.Value()),
		TamperPct:     float64(slTamper.Value()),
		ThrottleKbps:  slThrottle.Value(),
		Direction:     dir,
		Protocol:      proto,
		PortFilter:    port,
		IpFilter:      ip,
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fmtBytes(b int64) string {
	if b < 1024 {
		return strconv.FormatInt(b, 1) + " B"
	}
	if b < 1048576 {
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	}
	if b < 1073741824 {
		return fmt.Sprintf("%.1f MB", float64(b)/1048576)
	}
	return fmt.Sprintf("%.2f GB", float64(b)/1073741824)
}

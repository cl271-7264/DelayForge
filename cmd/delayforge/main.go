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
		walk.MsgBox(nil, "Error", "WinDivert.dll not found.", walk.MsgBoxIconError)
		return
	}
	if err := loadWinDivertDLL(dllPath); err != nil {
		walk.MsgBox(nil, "Error", fmt.Sprintf("Load WinDivert failed:\n%v", err), walk.MsgBoxIconError)
		return
	}
	log.Println("WinDivert loaded OK")
	engine = NewDamageEngine()

	var mw *walk.MainWindow
	var btnToggle *walk.PushButton
	var slLat, slJit, slLoss, slDup, slReorder, slTamper, slThrottle *walk.Slider
	var cmbDir, cmbProto *walk.ComboBox
	var leIP, lePort *walk.LineEdit
	var lblP, lblB, lblD, lblDr, lblDu, lblR, lblT *walk.Label

	err := MainWindow{
		AssignTo: &mw,
		Title:    "DelayForge",
		MinSize:  Size{460, 600},
		Size:     Size{480, 660},
		Layout:   VBox{},
		Children: []Widget{
			TabWidget{Pages: []TabPage{
				{Title: "Damage", Layout: VBox{Margins: Margins{Top: 8, Left: 12, Right: 12, Bottom: 8}}, Children: []Widget{
					GroupBox{Title: "Latency", Layout: Grid{Columns: 2, Spacing: 8}, Children: []Widget{
						Label{Text: "Base Delay:"}, Slider{AssignTo: &slLat, MinValue: 0, MaxValue: 3000},
						Label{Text: "Jitter (±):"}, Slider{AssignTo: &slJit, MinValue: 0, MaxValue: 500},
					}},
					GroupBox{Title: "Packet Loss", Layout: Grid{Columns: 2, Spacing: 8}, Children: []Widget{
						Label{Text: "Drop Rate:"}, Slider{AssignTo: &slLoss, MinValue: 0, MaxValue: 100},
					}},
					GroupBox{Title: "Duplicate", Layout: Grid{Columns: 2, Spacing: 8}, Children: []Widget{
						Label{Text: "Rate:"}, Slider{AssignTo: &slDup, MinValue: 0, MaxValue: 50},
					}},
					GroupBox{Title: "Reorder", Layout: Grid{Columns: 2, Spacing: 8}, Children: []Widget{
						Label{Text: "Rate:"}, Slider{AssignTo: &slReorder, MinValue: 0, MaxValue: 50},
					}},
					GroupBox{Title: "Tamper", Layout: Grid{Columns: 2, Spacing: 8}, Children: []Widget{
						Label{Text: "Corrupt Rate:"}, Slider{AssignTo: &slTamper, MinValue: 0, MaxValue: 50},
					}},
					GroupBox{Title: "Bandwidth Throttle", Layout: Grid{Columns: 2, Spacing: 8}, Children: []Widget{
						Label{Text: "Max kbps:"}, Slider{AssignTo: &slThrottle, MinValue: 0, MaxValue: 10000},
					}},
				}},
				{Title: "Filters", Layout: VBox{Margins: Margins{Top: 8, Left: 12, Right: 12, Bottom: 8}}, Children: []Widget{
					GroupBox{Title: "Filter Rules", Layout: Grid{Columns: 2, Spacing: 6}, Children: []Widget{
						Label{Text: "Direction:"}, ComboBox{AssignTo: &cmbDir, Model: []string{"Both", "Outbound", "Inbound"}, CurrentIndex: 0},
						Label{Text: "Protocol:"}, ComboBox{AssignTo: &cmbProto, Model: []string{"All", "TCP", "UDP", "ICMP"}, CurrentIndex: 0},
						Label{Text: "IP / CIDR:"}, LineEdit{AssignTo: &leIP},
						Label{Text: "Ports:"}, LineEdit{AssignTo: &lePort},
					}},
				}},
				{Title: "Stats", Layout: VBox{Margins: Margins{Top: 8, Left: 12, Right: 12, Bottom: 8}}, Children: []Widget{
					GroupBox{Title: "Live Statistics", Layout: Grid{Columns: 2, Spacing: 4}, Children: []Widget{
						Label{Text: "Processed:"}, Label{AssignTo: &lblP, Text: "0"},
						Label{Text: "Bytes:"}, Label{AssignTo: &lblB, Text: "0 B"},
						Label{Text: "Delayed:"}, Label{AssignTo: &lblD, Text: "0"},
						Label{Text: "Dropped:"}, Label{AssignTo: &lblDr, Text: "0"},
						Label{Text: "Duplicated:"}, Label{AssignTo: &lblDu, Text: "0"},
						Label{Text: "Reordered:"}, Label{AssignTo: &lblR, Text: "0"},
						Label{Text: "Tampered:"}, Label{AssignTo: &lblT, Text: "0"},
					}},
				}},
			}},
			Composite{Layout: HBox{}, Children: []Widget{
				HSpacer{},
				PushButton{AssignTo: &btnToggle, Text: "Start", OnClicked: func() {
					if engine.running {
						engine.Stop()
						if engine.handle != 0 {
							winDivertClose(engine.handle)
							engine.handle = 0
						}
						btnToggle.SetText("Start")
					} else {
						cfg := buildCfg(cmbDir, cmbProto, leIP, lePort, slLat, slJit, slLoss, slDup, slReorder, slTamper, slThrottle)
						filter := BuildFilter(cfg)
						log.Printf("Filter: %s", filter)
						handle, e := winDivertOpen(filter, WINDIVERT_LAYER_NETWORK, 0, WINDIVERT_FLAG_DEFAULT)
						if e != nil {
							walk.MsgBox(mw, "Error", fmt.Sprintf("WinDivertOpen failed:\n%v\n\nRun as Administrator!", e), walk.MsgBoxIconError)
							return
						}
						engine.Start(handle, cfg)
						btnToggle.SetText("Stop")
					}
				}},
				HSpacer{},
			}},
		},
	}.Create()

	if err != nil {
		log.Printf("UI error: %v", err)
		walk.MsgBox(nil, "Error", fmt.Sprintf("UI error:\n%v", err), walk.MsgBoxIconError)
		return
	}

	go func() {
		t := time.NewTicker(500 * time.Millisecond)
		for range t.C {
			if mw == nil || mw.IsDisposed() {
				return
			}
			mw.Synchronize(func() {
				lblP.SetText(strconv.FormatInt(engine.stats.Processed.Load(), 1))
				lblB.SetText(fmtBytes(engine.stats.Bytes.Load()))
				lblD.SetText(strconv.FormatInt(engine.stats.Delayed.Load(), 1))
				lblDr.SetText(strconv.FormatInt(engine.stats.Dropped.Load(), 1))
				lblDu.SetText(strconv.FormatInt(engine.stats.Duplicated.Load(), 1))
				lblR.SetText(strconv.FormatInt(engine.stats.Reordered.Load(), 1))
				lblT.SetText(strconv.FormatInt(engine.stats.Tampered.Load(), 1))
			})
			if engine.running {
				engine.UpdateConfig(buildCfg(cmbDir, cmbProto, leIP, lePort, slLat, slJit, slLoss, slDup, slReorder, slTamper, slThrottle))
			}
		}
	}()

	mw.Run()
}

func buildCfg(cmbDir, cmbProto *walk.ComboBox, leIP, lePort *walk.LineEdit,
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
		LatencyMs: slLat.Value(), JitterMs: slJit.Value(),
		PacketLossPct: float64(slLoss.Value()), DuplicatePct: float64(slDup.Value()),
		ReorderPct: float64(slReorder.Value()), TamperPct: float64(slTamper.Value()),
		ThrottleKbps: slThrottle.Value(), Direction: dir, Protocol: proto,
		PortFilter: port, IpFilter: ip,
	}
}

func fileExists(p string) bool { _, e := os.Stat(p); return e == nil }

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

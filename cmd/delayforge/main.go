package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

func init() {
	user32 := syscall.NewLazyDLL("user32.dll")
	user32.NewProc("SetProcessDPIAware").Call()
}

var (
	engine  *DamageEngine
	dllPath string
	lang    = "en" // "en" or "zh"
)

// Bilingual labels
var L = map[string]map[string]string{
	"en": {
		"title":    "DelayForge",
		"damage":   "Damage",
		"filters":  "Filters",
		"stats":    "Statistics",
		"latency":  "Latency",
		"jitter":   "Jitter (±)",
		"loss":     "Packet Loss",
		"dup":      "Duplicate",
		"reorder":  "Reorder",
		"tamper":   "Tamper (Corrupt)",
		"throttle": "Bandwidth Throttle",
		"rate":     "Rate",
		"drop":     "Drop Rate",
		"corrupt":  "Corrupt Rate",
		"max":      "Max Bandwidth",
		"dir":      "Direction",
		"proto":    "Protocol",
		"ip":       "IP / CIDR",
		"port":     "Ports",
		"both":     "Both",
		"out":      "Outbound",
		"in":       "Inbound",
		"all":      "All",
		"start":    "▶ Start",
		"stop":     "⏹ Stop",
		"proc":     "Processed",
		"bytes":    "Bytes",
		"delayed":  "Delayed",
		"dropped":  "Dropped",
		"duplicated": "Duplicated",
		"reordered":  "Reordered",
		"tampered":   "Tampered",
		"live":     "Live Statistics",
		"lang":     "中文",
	},
	"zh": {
		"title":    "DelayForge 延迟锻造",
		"damage":   "损伤参数",
		"filters":  "过滤规则",
		"stats":    "实时统计",
		"latency":  "延迟",
		"jitter":   "抖动 (±)",
		"loss":     "丢包",
		"dup":      "重复",
		"reorder":  "乱序",
		"tamper":   "篡改 (损坏)",
		"throttle": "带宽限速",
		"rate":     "比例",
		"drop":     "丢包率",
		"corrupt":  "篡改率",
		"max":      "最大带宽",
		"dir":      "方向",
		"proto":    "协议",
		"ip":       "IP / CIDR",
		"port":     "端口",
		"both":     "双向",
		"out":      "仅出站",
		"in":       "仅入站",
		"all":      "全部",
		"start":    "▶ 启动",
		"stop":     "⏹ 停止",
		"proc":     "已处理",
		"bytes":    "字节数",
		"delayed":  "已延迟",
		"dropped":  "已丢弃",
		"duplicated": "已复制",
		"reordered":  "已乱序",
		"tampered":   "已篡改",
		"live":     "实时统计",
		"lang":     "EN",
	},
}

func t(key string) string {
	if m, ok := L[lang]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return key
}

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
	var btnToggle, btnLang *walk.PushButton
	var slLat, slJit, slLoss, slDup, slReorder, slTamper, slThrottle *walk.Slider
	var cmbDir, cmbProto *walk.ComboBox
	var leIP, lePort *walk.LineEdit
	var lblP, lblB, lblD, lblDr, lblDu, lblR, lblT *walk.Label

	err := MainWindow{
		AssignTo: &mw,
		Title:    t("title"),
		MinSize:  Size{480, 640},
		Size:     Size{500, 700},
		Layout:   VBox{},
		Children: []Widget{
			// Language toggle
			Composite{Layout: HBox{}, Children: []Widget{
				HSpacer{},
				PushButton{AssignTo: &btnLang, Text: t("lang"), OnClicked: func() {
					if lang == "en" {
						lang = "zh"
					} else {
						lang = "en"
					}
					mw.SetTitle(t("title"))
					btnLang.SetText(t("lang"))
				}},
			}},
			TabWidget{Pages: []TabPage{
				// === Damage ===
				{Title: t("damage"), Layout: VBox{Margins: Margins{Top: 10, Left: 16, Right: 16, Bottom: 10}}, Children: []Widget{
					GroupBox{Title: t("latency"), Layout: Grid{Columns: 3, Spacing: 8, Margins: Margins{Left: 8, Right: 8}}, Children: []Widget{
						Label{Text: t("latency") + ":"}, Slider{AssignTo: &slLat, MinValue: 0, MaxValue: 3000}, Label{Text: "ms"},
					}},
					GroupBox{Title: t("jitter"), Layout: Grid{Columns: 3, Spacing: 8, Margins: Margins{Left: 8, Right: 8}}, Children: []Widget{
						Label{Text: t("jitter") + ":"}, Slider{AssignTo: &slJit, MinValue: 0, MaxValue: 500}, Label{Text: "ms"},
					}},
					GroupBox{Title: t("loss"), Layout: Grid{Columns: 3, Spacing: 8, Margins: Margins{Left: 8, Right: 8}}, Children: []Widget{
						Label{Text: t("drop") + ":"}, Slider{AssignTo: &slLoss, MinValue: 0, MaxValue: 100}, Label{Text: "%"},
					}},
					GroupBox{Title: t("dup"), Layout: Grid{Columns: 3, Spacing: 8, Margins: Margins{Left: 8, Right: 8}}, Children: []Widget{
						Label{Text: t("rate") + ":"}, Slider{AssignTo: &slDup, MinValue: 0, MaxValue: 50}, Label{Text: "%"},
					}},
					GroupBox{Title: t("reorder"), Layout: Grid{Columns: 3, Spacing: 8, Margins: Margins{Left: 8, Right: 8}}, Children: []Widget{
						Label{Text: t("rate") + ":"}, Slider{AssignTo: &slReorder, MinValue: 0, MaxValue: 50}, Label{Text: "%"},
					}},
					GroupBox{Title: t("tamper"), Layout: Grid{Columns: 3, Spacing: 8, Margins: Margins{Left: 8, Right: 8}}, Children: []Widget{
						Label{Text: t("corrupt") + ":"}, Slider{AssignTo: &slTamper, MinValue: 0, MaxValue: 50}, Label{Text: "%"},
					}},
					GroupBox{Title: t("throttle"), Layout: Grid{Columns: 3, Spacing: 8, Margins: Margins{Left: 8, Right: 8}}, Children: []Widget{
						Label{Text: t("max") + ":"}, Slider{AssignTo: &slThrottle, MinValue: 0, MaxValue: 10000}, Label{Text: "kbps"},
					}},
				}},
				// === Filters ===
				{Title: t("filters"), Layout: VBox{Margins: Margins{Top: 10, Left: 16, Right: 16, Bottom: 10}}, Children: []Widget{
					GroupBox{Title: t("filters"), Layout: Grid{Columns: 2, Spacing: 8, Margins: Margins{Left: 8, Right: 8, Top: 8, Bottom: 8}}, Children: []Widget{
						Label{Text: t("dir") + ":"}, ComboBox{AssignTo: &cmbDir, Model: []string{t("both"), t("out"), t("in")}, CurrentIndex: 0},
						Label{Text: t("proto") + ":"}, ComboBox{AssignTo: &cmbProto, Model: []string{t("all"), "TCP", "UDP", "ICMP"}, CurrentIndex: 0},
						Label{Text: t("ip") + ":"}, LineEdit{AssignTo: &leIP},
						Label{Text: t("port") + ":"}, LineEdit{AssignTo: &lePort},
					}},
				}},
				// === Stats ===
				{Title: t("stats"), Layout: VBox{Margins: Margins{Top: 10, Left: 16, Right: 16, Bottom: 10}}, Children: []Widget{
					GroupBox{Title: t("live"), Layout: Grid{Columns: 2, Spacing: 6, Margins: Margins{Left: 8, Right: 8, Top: 8, Bottom: 8}}, Children: []Widget{
						Label{Text: t("proc") + ":"}, Label{AssignTo: &lblP, Text: "0"},
						Label{Text: t("bytes") + ":"}, Label{AssignTo: &lblB, Text: "0 B"},
						Label{Text: t("delayed") + ":"}, Label{AssignTo: &lblD, Text: "0"},
						Label{Text: t("dropped") + ":"}, Label{AssignTo: &lblDr, Text: "0"},
						Label{Text: t("duplicated") + ":"}, Label{AssignTo: &lblDu, Text: "0"},
						Label{Text: t("reordered") + ":"}, Label{AssignTo: &lblR, Text: "0"},
						Label{Text: t("tampered") + ":"}, Label{AssignTo: &lblT, Text: "0"},
					}},
				}},
			}},
			// === Start/Stop ===
			Composite{Layout: HBox{Margins: Margins{Top: 4, Bottom: 8}}, Children: []Widget{
				HSpacer{},
				PushButton{AssignTo: &btnToggle, Text: t("start"), MinSize: Size{120, 36}, OnClicked: func() {
					if engine.running {
						engine.Stop()
						if engine.handle != 0 {
							winDivertClose(engine.handle)
							engine.handle = 0
						}
						btnToggle.SetText(t("start"))
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
						btnToggle.SetText(t("stop"))
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

	// Stats refresh
	go func() {
		tk := time.NewTicker(500 * time.Millisecond)
		for range tk.C {
			if mw == nil || mw.IsDisposed() {
				return
			}
			mw.Synchronize(func() {
				lblP.SetText(strconv.FormatInt(engine.stats.Processed.Load(), 10))
				lblB.SetText(fmtBytes(engine.stats.Bytes.Load()))
				lblD.SetText(strconv.FormatInt(engine.stats.Delayed.Load(), 10))
				lblDr.SetText(strconv.FormatInt(engine.stats.Dropped.Load(), 10))
				lblDu.SetText(strconv.FormatInt(engine.stats.Duplicated.Load(), 10))
				lblR.SetText(strconv.FormatInt(engine.stats.Reordered.Load(), 10))
				lblT.SetText(strconv.FormatInt(engine.stats.Tampered.Load(), 10))
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
		return strconv.FormatInt(b, 10) + " B"
	}
	if b < 1048576 {
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	}
	if b < 1073741824 {
		return fmt.Sprintf("%.1f MB", float64(b)/1048576)
	}
	return fmt.Sprintf("%.2f GB", float64(b)/1073741824)
}

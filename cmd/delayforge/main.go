package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func init() {
	syscall.NewLazyDLL("user32.dll").NewProc("SetProcessDPIAware").Call()
}

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
		fmt.Println("WinDivert.dll not found!")
		return
	}
	if err := loadWinDivertDLL(dllPath); err != nil {
		fmt.Printf("Load WinDivert failed: %v\n", err)
		return
	}
	log.Println("WinDivert loaded OK")
	engine = NewDamageEngine()

	myApp := app.New()
	myWindow := myApp.NewWindow("DelayForge")
	myWindow.Resize(fyne.NewSize(520, 700))

	// --- Sliders ---
	slLat := widget.NewSlider(0, 3000)
	slJit := widget.NewSlider(0, 500)
	slLoss := widget.NewSlider(0, 100)
	slDup := widget.NewSlider(0, 50)
	slReorder := widget.NewSlider(0, 50)
	slTamper := widget.NewSlider(0, 50)
	slThrottle := widget.NewSlider(0, 10000)
	slThrottle.Step = 10

	// --- Labels for slider values ---
	lblLat := widget.NewLabel("0 ms")
	lblJit := widget.NewLabel("0 ms")
	lblLoss := widget.NewLabel("0 %")
	lblDup := widget.NewLabel("0 %")
	lblReorder := widget.NewLabel("0 %")
	lblTamper := widget.NewLabel("0 %")
	lblThrottle := widget.NewLabel("0 kbps")

	slLat.OnChanged = func(v float64) { lblLat.SetText(fmt.Sprintf("%d ms", int(v))) }
	slJit.OnChanged = func(v float64) { lblJit.SetText(fmt.Sprintf("%d ms", int(v))) }
	slLoss.OnChanged = func(v float64) { lblLoss.SetText(fmt.Sprintf("%d %%", int(v))) }
	slDup.OnChanged = func(v float64) { lblDup.SetText(fmt.Sprintf("%d %%", int(v))) }
	slReorder.OnChanged = func(v float64) { lblReorder.SetText(fmt.Sprintf("%d %%", int(v))) }
	slTamper.OnChanged = func(v float64) { lblTamper.SetText(fmt.Sprintf("%d %%", int(v))) }
	slThrottle.OnChanged = func(v float64) { lblThrottle.SetText(fmt.Sprintf("%d kbps", int(v))) }

	// --- Filters ---
	cmbDir := widget.NewSelect([]string{"双向 Both", "出站 Outbound", "入站 Inbound"}, nil)
	cmbDir.SetSelected("双向 Both")
	cmbProto := widget.NewSelect([]string{"全部 All", "TCP", "UDP", "ICMP"}, nil)
	cmbProto.SetSelected("全部 All")
	leIP := widget.NewEntry()
	leIP.SetPlaceHolder("e.g. 192.168.1.0/24")
	lePort := widget.NewEntry()
	lePort.SetPlaceHolder("e.g. 80, 443")

	// --- Stats ---
	statP := widget.NewLabel("0")
	statB := widget.NewLabel("0 B")
	statD := widget.NewLabel("0")
	statDr := widget.NewLabel("0")
	statDu := widget.NewLabel("0")
	statR := widget.NewLabel("0")
	statT := widget.NewLabel("0")

	// --- Start/Stop ---
	running := false
	btn := widget.NewButtonWithIcon("启动 Start", theme.MediaPlayIcon(), func() {})
	btn.Importance = widget.HighImportance

	btn.OnTapped = func() {
		if running {
			engine.Stop()
			if engine.handle != 0 {
				winDivertClose(engine.handle)
				engine.handle = 0
			}
			running = false
			btn.SetText("启动 Start")
			btn.SetIcon(theme.MediaPlayIcon())
		} else {
			cfg := getCfg(cmbDir, cmbProto, leIP, lePort, slLat, slJit, slLoss, slDup, slReorder, slTamper, slThrottle)
			filter := BuildFilter(cfg)
			log.Printf("Filter: %s", filter)
			handle, e := winDivertOpen(filter, WINDIVERT_LAYER_NETWORK, 0, WINDIVERT_FLAG_DEFAULT)
			if e != nil {
				dialog.ShowError(fmt.Errorf("WinDivertOpen failed:\n%v\n\nRun as Administrator!", e), myWindow)
				return
			}
			engine.Start(handle, cfg)
			running = true
			btn.SetText("停止 Stop")
			btn.SetIcon(theme.MediaStopIcon())
		}
	}

	// --- Build UI ---
	makeSlider := func(label string, sl *widget.Slider, lbl *widget.Label) fyne.CanvasObject {
		return container.NewBorder(nil, nil, widget.NewLabel(label), lbl, sl)
	}

	damageTab := container.NewVBox(
		widget.NewLabel("损伤 Damage Parameters"),
		widget.NewSeparator(),
		makeSlider("延迟 Latency:", slLat, lblLat),
		makeSlider("抖动 Jitter:", slJit, lblJit),
		makeSlider("丢包 Loss:", slLoss, lblLoss),
		makeSlider("重复 Dup:", slDup, lblDup),
		makeSlider("乱序 Reorder:", slReorder, lblReorder),
		makeSlider("篡改 Tamper:", slTamper, lblTamper),
		makeSlider("限速 Throttle:", slThrottle, lblThrottle),
	)

	filterTab := container.NewVBox(
		widget.NewLabel("过滤规则 Filter Rules"),
		widget.NewSeparator(),
		widget.NewLabel("方向 Direction:"), cmbDir,
		widget.NewLabel("协议 Protocol:"), cmbProto,
		widget.NewLabel("IP / CIDR:"), leIP,
		widget.NewLabel("端口 Ports:"), lePort,
	)

	statsTab := container.NewVBox(
		widget.NewLabel("实时统计 Live Statistics"),
		widget.NewSeparator(),
		container.NewGridWithColumns(2,
			widget.NewLabel("已处理 Processed:"), statP,
			widget.NewLabel("字节数 Bytes:"), statB,
			widget.NewLabel("已延迟 Delayed:"), statD,
			widget.NewLabel("已丢弃 Dropped:"), statDr,
			widget.NewLabel("已复制 Duplicated:"), statDu,
			widget.NewLabel("已乱序 Reordered:"), statR,
			widget.NewLabel("已篡改 Tampered:"), statT,
		),
	)

	tabs := container.NewAppTabs(
		container.NewTabItem("损伤 Damage", damageTab),
		container.NewTabItem("过滤 Filters", filterTab),
		container.NewTabItem("统计 Stats", statsTab),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	content := container.NewBorder(nil, container.NewPadded(btn), nil, nil, tabs)
	myWindow.SetContent(content)

	// Stats refresh
	go func() {
		tk := time.NewTicker(500 * time.Millisecond)
		for range tk.C {
			statP.SetText(strconv.FormatInt(engine.stats.Processed.Load(), 10))
			statB.SetText(fmtBytes(engine.stats.Bytes.Load()))
			statD.SetText(strconv.FormatInt(engine.stats.Delayed.Load(), 10))
			statDr.SetText(strconv.FormatInt(engine.stats.Dropped.Load(), 10))
			statDu.SetText(strconv.FormatInt(engine.stats.Duplicated.Load(), 10))
			statR.SetText(strconv.FormatInt(engine.stats.Reordered.Load(), 10))
			statT.SetText(strconv.FormatInt(engine.stats.Tampered.Load(), 10))
			if running {
				engine.UpdateConfig(getCfg(cmbDir, cmbProto, leIP, lePort, slLat, slJit, slLoss, slDup, slReorder, slTamper, slThrottle))
			}
		}
	}()

	myWindow.ShowAndRun()
}

func getCfg(cmbDir *widget.Select, cmbProto *widget.Select, leIP, lePort *widget.Entry,
	slLat, slJit, slLoss, slDup, slReorder, slTamper, slThrottle *widget.Slider) DamageConfig {
	dir := "both"
	switch cmbDir.Selected {
	case "出站 Outbound":
		dir = "outbound"
	case "入站 Inbound":
		dir = "inbound"
	}
	proto := "any"
	switch cmbProto.Selected {
	case "TCP":
		proto = "tcp"
	case "UDP":
		proto = "udp"
	case "ICMP":
		proto = "icmp"
	}
	return DamageConfig{
		LatencyMs: int(slLat.Value), JitterMs: int(slJit.Value),
		PacketLossPct: slLoss.Value, DuplicatePct: slDup.Value,
		ReorderPct: slReorder.Value, TamperPct: slTamper.Value,
		ThrottleKbps: int(slThrottle.Value), Direction: dir, Protocol: proto,
		PortFilter: lePort.Text, IpFilter: leIP.Text,
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

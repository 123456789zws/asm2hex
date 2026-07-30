package main

/*
#cgo CFLAGS: -O2 -Wall
*/
import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/color"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/suifei/asm2hex/archs"
	"github.com/suifei/asm2hex/bindings/capstone"
	"github.com/suifei/asm2hex/bindings/keystone"
	"github.com/suifei/asm2hex/theme/icons"
)

const (
	ApplicationTitle       = "ASM to HEX Converter"
	ApplicationTitleToggle = "HEX to ASM Converter"
)

type ToggleMode string

const (
	ASM2HEX ToggleMode = "asm2hex"
	HEX2ASM ToggleMode = "hex2asm"
)

var toggle_mode ToggleMode = ASM2HEX
var codes []string = nil
var prefix_hex bool = false
var offset uint64 = 0
var bigEndian bool = false
var addAddress bool = false

type Param struct {
	Arch   uint64
	Mode   uint64
	Syntax int
	Info   string
}

var KSSelectParam = &Param{
	Arch:   0,
	Mode:   0,
	Syntax: -1,
	Info:   "",
}
var CSSelectParam = &Param{
	Arch:   0,
	Mode:   0,
	Syntax: -1,
	Info:   "",
}

var status *widget.Label
var convertBtn *widget.Button
var toggleBtn *widget.Button
var keystoneArchDropdown,
	keystoneModeDropdown,
	keystoneSyntaxDropdown,
	capstoneArchDropdown,
	capstoneModeDropdown,
	capstoneSyntaxDropdown *widget.Select

var asm2hexTools *fyne.Container
var hex2asmTools *fyne.Container
var output1_info *widget.Label

// Widgets used by background conversion (set in createMainUI).
var convStatus *widget.Label
var convOutput *widget.Entry
var convEditor *widget.Entry
var convOffset *widget.Entry

// convertMu serializes conversions; convertSeq drops stale async results.
var convertMu sync.Mutex
var convertSeq atomic.Uint64
var convertDebounceMu sync.Mutex
var convertDebounceTimer *time.Timer

const convertDebounceDelay = 200 * time.Millisecond

func getOptionNames(options archs.OptionSlice) []string {
	names := make([]string, len(options))
	for i, option := range options {
		names[i] = option.Name
	}
	return names
}
func toJson(o interface{}) string {
	buf, _ := json.MarshalIndent(o, "", "  ")
	return string(buf)
}
func updateSelectParam() {
	if keystoneArchDropdown != nil && keystoneArchDropdown.SelectedIndex() != -1 {
		mapKey := archs.KeystoneArchOptions[keystoneArchDropdown.SelectedIndex()].Const
		KSSelectParam.Arch = mapKey
		KSSelectParam.Info = keystoneArchDropdown.Selected
	}
	if capstoneArchDropdown != nil && capstoneArchDropdown.SelectedIndex() != -1 {
		mapKey := archs.CapstoneArchOptions[capstoneArchDropdown.SelectedIndex()].Const
		CSSelectParam.Arch = mapKey
		CSSelectParam.Info = capstoneArchDropdown.Selected
	}
	if keystoneModeDropdown != nil && archs.KeystoneModeList != nil {
		index := keystoneModeDropdown.SelectedIndex()
		if index >= 0 && index < len(archs.KeystoneModeList) {
			KSSelectParam.Mode = archs.KeystoneModeList[index].Const
			KSSelectParam.Info += " " + keystoneModeDropdown.Selected
		}
	}
	if capstoneModeDropdown != nil && archs.CapstoneModeList != nil {
		index := capstoneModeDropdown.SelectedIndex()
		if index >= 0 && index < len(archs.CapstoneModeList) {
			CSSelectParam.Mode = archs.CapstoneModeList[index].Const
			CSSelectParam.Info += " " + capstoneModeDropdown.Selected
		}
	}
	if keystoneSyntaxDropdown != nil && archs.KeystoneSyntaxList != nil && len(archs.KeystoneSyntaxList) > 0 {
		index := keystoneSyntaxDropdown.SelectedIndex()
		if index >= 0 && index < len(archs.KeystoneSyntaxList) {
			KSSelectParam.Syntax = int(archs.KeystoneSyntaxList[index].Const)
			KSSelectParam.Info += " " + keystoneSyntaxDropdown.Selected
		}
	} else {
		KSSelectParam.Syntax = -1
	}
	if capstoneSyntaxDropdown != nil && archs.CapstoneSyntaxList != nil {
		index := capstoneSyntaxDropdown.SelectedIndex()
		if index >= 0 && index < len(archs.CapstoneSyntaxList) {
			CSSelectParam.Syntax = int(archs.CapstoneSyntaxList[index].Const)
			CSSelectParam.Info += " " + capstoneSyntaxDropdown.Selected
		}
	}

	// fmt.Println("Keystone", toJson(KSSelectParam))
	// fmt.Println("Capstone", toJson(CSSelectParam))

	if toggle_mode == ASM2HEX {
		output1_info.Text = KSSelectParam.Info
	} else {
		output1_info.Text = CSSelectParam.Info
	}

	if output1_info != nil {
		output1_info.Refresh()
	}
	scheduleConversion()
}

func conversionReady() bool {
	return convStatus != nil && convOutput != nil && convEditor != nil && convOffset != nil
}

// scheduleConversion debounces auto-convert so rapid typing/paste does not
// flood the assembler or freeze the UI thread.
func scheduleConversion() {
	if !conversionReady() {
		return
	}
	convertDebounceMu.Lock()
	defer convertDebounceMu.Unlock()
	if convertDebounceTimer != nil {
		convertDebounceTimer.Stop()
	}
	convertDebounceTimer = time.AfterFunc(convertDebounceDelay, func() {
		// Run conversion off the UI thread so a slow/bad keystone call cannot freeze UI.
		go doConversion(convStatus, convOutput, convEditor, convOffset)
	})
}

// triggerConversion runs conversion immediately (Convert button / mode toggle).
func triggerConversion() {
	if !conversionReady() {
		return
	}
	convertDebounceMu.Lock()
	if convertDebounceTimer != nil {
		convertDebounceTimer.Stop()
		convertDebounceTimer = nil
	}
	convertDebounceMu.Unlock()
	go doConversion(convStatus, convOutput, convEditor, convOffset)
}
func createDropdowns() *fyne.Container {
	keystoneArchDropdown = &widget.Select{}
	keystoneArchDropdown.ExtendBaseWidget(keystoneArchDropdown)
	keystoneArchDropdown.SetOptions(getOptionNames(archs.KeystoneArchOptions))
	keystoneArchDropdown.OnChanged = func(s string) {
		// fmt.Println("Keystone Arch:", s)
		// fmt.Println(keystoneArchDropdown.SelectedIndex())
		mapKey := archs.KeystoneArchOptions[keystoneArchDropdown.SelectedIndex()].Const
		if options, ok := archs.KeystoneModeOptions[mapKey]; ok && keystoneModeDropdown != nil {
			archs.KeystoneModeList = options
			keystoneModeDropdown.SetOptions(getOptionNames(options))
			keystoneModeDropdown.SetSelectedIndex(0)
		}
		if options, ok := archs.KeystoneSyntaxOptions[mapKey]; ok && keystoneSyntaxDropdown != nil {
			archs.KeystoneSyntaxList = options
			names := getOptionNames(options)
			keystoneSyntaxDropdown.SetOptions(names)
			if len(names) > 0 {
				keystoneSyntaxDropdown.SetSelectedIndex(0)
			} else {
				keystoneSyntaxDropdown.Selected = ""
				keystoneSyntaxDropdown.Refresh()
				KSSelectParam.Syntax = -1
			}
		} else if keystoneSyntaxDropdown != nil {
			archs.KeystoneSyntaxList = nil
			keystoneSyntaxDropdown.SetOptions([]string{})
			keystoneSyntaxDropdown.Selected = ""
			keystoneSyntaxDropdown.Refresh()
			KSSelectParam.Syntax = -1
		}
		updateSelectParam()
	}

	keystoneModeDropdown = &widget.Select{}
	keystoneModeDropdown.ExtendBaseWidget(keystoneModeDropdown)
	keystoneModeDropdown.OnChanged = func(s string) {
		// fmt.Println("Keystone Mode:", s)
		updateSelectParam()
	}

	keystoneSyntaxDropdown = &widget.Select{}
	keystoneSyntaxDropdown.ExtendBaseWidget(keystoneSyntaxDropdown)
	keystoneSyntaxDropdown.OnChanged = func(s string) {
		fmt.Println("Keystone Syntax:", s)
		updateSelectParam()
	}

	capstoneArchDropdown = &widget.Select{}
	capstoneArchDropdown.ExtendBaseWidget(capstoneArchDropdown)
	capstoneArchDropdown.SetOptions(getOptionNames(archs.CapstoneArchOptions))
	capstoneArchDropdown.OnChanged = func(s string) {
		// fmt.Println("Capstone Arch:", s)
		// fmt.Println(capstoneArchDropdown.SelectedIndex())
		mapKey := archs.CapstoneArchOptions[capstoneArchDropdown.SelectedIndex()].Const
		if options, ok := archs.CapstoneModeOptions[mapKey]; ok && capstoneModeDropdown != nil {
			archs.CapstoneModeList = options
			capstoneModeDropdown.SetOptions(getOptionNames(options))
			capstoneModeDropdown.SetSelectedIndex(0)
		}
		if options, ok := archs.CapstoneSyntaxOptions[mapKey]; ok && capstoneSyntaxDropdown != nil {
			archs.CapstoneSyntaxList = options
			capstoneSyntaxDropdown.SetOptions(getOptionNames(options))
			capstoneSyntaxDropdown.SetSelectedIndex(0)
		}
		updateSelectParam()
	}
	capstoneModeDropdown = &widget.Select{}
	capstoneModeDropdown.ExtendBaseWidget(capstoneModeDropdown)
	capstoneModeDropdown.OnChanged = func(s string) {
		// fmt.Println("Capstone Mode:", s)
		updateSelectParam()
	}

	capstoneSyntaxDropdown = &widget.Select{}
	capstoneSyntaxDropdown.ExtendBaseWidget(capstoneSyntaxDropdown)
	capstoneSyntaxDropdown.OnChanged = func(s string) {
		fmt.Println("Capstone Syntax:", s)
		updateSelectParam()
	}

	keystoneArchDropdown.SetSelectedIndex(1)
	capstoneArchDropdown.SetSelectedIndex(1)
	asm2hexTools = container.NewHBox(
		keystoneArchDropdown,
		keystoneModeDropdown,
		keystoneSyntaxDropdown,
	)
	hex2asmTools =
		container.NewHBox(
			capstoneArchDropdown,
			capstoneModeDropdown,
			capstoneSyntaxDropdown,
		)
	asm2hexTools.Show()
	hex2asmTools.Hidden = true
	asm2hexTools.Refresh()
	hex2asmTools.Refresh()

	return container.NewVBox(
		asm2hexTools, hex2asmTools,
	)

}

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow(ApplicationTitle)

	myWindow.SetContent(createMainUI(myWindow))

	myWindow.Resize(fyne.NewSize(800, 600))

	go downloadWebResource()

	myWindow.ShowAndRun()
}
func downloadWebResource() {
	if icons.CAPSTONE_PNG_RES == nil || icons.KEYSTONE_PNG_RES == nil {
		go func() {
			cs, err := fyne.LoadResourceFromURLString("https://www.capstone-engine.org/img/capstone.png")
			if err != nil {
				fmt.Println(err)
			}
			icons.CAPSTONE_PNG_RES = cs
		}()
		go func() {
			ks, err := fyne.LoadResourceFromURLString("https://www.keystone-engine.org/images/keystone.png")
			if err != nil {
				fmt.Println(err)
			}
			icons.KEYSTONE_PNG_RES = ks
		}()
	}
}
func hexdump(buf []byte) string {
	if len(buf) == 0 {
		return ""
	}
	var hexStr string
	for _, b := range buf {
		hexStr += fmt.Sprintf("%02X", b)
	}
	if prefix_hex {
		return fmt.Sprintf("0x%s", hexStr)
	}
	return hexStr
}
func hexStringToBytes(s string) ([]byte, error) {
	s = strings.ReplaceAll(s, " ", "")
	bytes, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}
func createMainUI(win fyne.Window) *fyne.Container {

	assemblyLabel := widget.NewLabel("Assembly code")
	offsetLabel := widget.NewLabel("Offset(hex)")

	output1 := widget.NewMultiLineEntry()
	output1.SetPlaceHolder("ARM64")
	output1.SetMinRowsVisible(24)
	output1.TextStyle.Monospace = true

	output1_info = widget.NewLabel("Little Endian")

	output1_card := container.NewVBox(output1,
		container.NewHBox(
			output1_info,
			layout.NewSpacer(),
			widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
				win.Clipboard().SetContent(output1.Text)
				fyne.CurrentApp().SendNotification(&fyne.Notification{Title: "Info", Content: "Copied to clipboard"})
			}),
		))

	updateSelectParam()

	assembly_code := `; sample code
nop
ret
b #0x1018de444
mov x0, #0x11fe0000
beq #0x10020c
cbnz r0, #0x682c4
`

	assemblyEditor := widget.NewMultiLineEntry()
	assemblyEditor.SetPlaceHolder("Assembly code")
	assemblyEditor.SetText(assembly_code)
	assemblyEditor.AcceptsTab()
	assemblyEditor.SetMinRowsVisible(20)

	offsetInput := widget.NewEntry()
	offsetInput.SetPlaceHolder("Offset(hex)")
	offsetInput.SetText("0")

	assemblyEditor.OnChanged = func(text string) {
		scheduleConversion()
	}
	offsetInput.OnChanged = func(text string) {
		if text == "" {
			offset = 0
		} else {
			num, err := strconv.ParseUint(text, 16, 64)
			if err != nil {
				offset = 0
				status.SetText("Invalid offset")
			} else {
				offset = num
			}
		}
		status.SetText(fmt.Sprintf("Offset: 0x%x", offset))
		scheduleConversion()
		status.Refresh()
	}

	left_container := container.New(layout.NewVBoxLayout(),
		assemblyLabel,
		assemblyEditor,
		container.New(layout.NewFormLayout(), offsetLabel, offsetInput),
	)

	right_container := container.New(layout.NewVBoxLayout(),
		createDropdowns(),
		output1_card,
	)

	grid := container.New(layout.NewGridLayoutWithColumns(2),
		left_container,
		right_container,
	)

	logo_icon := fyne.NewStaticResource("asm2hex.svg", icons.LOGO_ICON_BIN)
	github_icon := fyne.NewStaticResource("github.svg", icons.GITHUB_ICON_BIN)

	background := canvas.NewImageFromResource(logo_icon)
	background.SetMinSize(fyne.NewSize(64, 64))

	app_title := canvas.NewText(ApplicationTitle, color.NRGBA{0, 0x80, 0, 0xff})
	app_title.TextSize = 24

	app_ver := fyne.CurrentApp().Metadata().Version
	if archs.WithRiscv {
		app_ver += " (RISC-V)"
	}
	about_messages := "ASM to HEX Converter\n\n" +
		"Version: " + app_ver + "\n" +
		"Author: suifei suifei@gmail.com\n" +
		"License: MIT\n" +
		"Source code: https://github.com/suifei/asm2hex\n\n" +
		"Copyright (c) 2024 suifei"
	// 关注按钮
	openUrlBtn := widget.NewButtonWithIcon("Star", logo_icon, func() {
		uri, _ := url.Parse("https://github.com/suifei/asm2hex")
		fyne.CurrentApp().OpenURL(uri)
	})
	openGithub := widget.NewButtonWithIcon("Github", github_icon, func() {
		uri, _ := url.Parse("https://github.com/suifei")
		fyne.CurrentApp().OpenURL(uri)
	})
	openFyne := widget.NewButtonWithIcon(fmt.Sprintf("Fyne %s", "v2.4.5"), theme.FyneLogo(), func() {
		uri, _ := url.Parse("https://fyne.io/")
		fyne.CurrentApp().OpenURL(uri)
	})
	openCapstone := widget.NewButtonWithIcon(fmt.Sprintf("Capstone v%d.%d", capstone.API_MAJOR, capstone.API_MINOR), icons.CAPSTONE_PNG_RES, func() {
		uri, _ := url.Parse("https://www.capstone-engine.org/")
		fyne.CurrentApp().OpenURL(uri)
	})
	openKeystone := widget.NewButtonWithIcon(fmt.Sprintf("Keystone v%d.%d", keystone.API_MAJOR, keystone.API_MINOR), icons.KEYSTONE_PNG_RES, func() {
		uri, _ := url.Parse("https://www.keystone-engine.org/")
		fyne.CurrentApp().OpenURL(uri)
	})
	aboutDlg := dialog.NewCustom("About", "Close", container.New(
		layout.NewVBoxLayout(), widget.NewLabel(about_messages), container.NewHBox(openUrlBtn, openGithub, openFyne, openCapstone, openKeystone),
	), win)
	status = widget.NewLabel("Ready")
	status.Alignment = fyne.TextAlignTrailing
	status.TextStyle = fyne.TextStyle{Bold: true}

	// Wire globals used by debounced / background conversion.
	convStatus = status
	convOutput = output1
	convEditor = assemblyEditor
	convOffset = offsetInput

	convertBtn = widget.NewButtonWithIcon("Convert", theme.StorageIcon(), func() {
		triggerConversion()
	})
	convertBtn.Importance = widget.HighImportance
	toggleBtn = widget.NewButtonWithIcon("Toggle Mode", theme.ViewRefreshIcon(), func() {
		if toggle_mode == ASM2HEX {
			toggle_mode = HEX2ASM
			app_title.Text = ApplicationTitleToggle
			app_title.Color = color.NRGBA{0x80, 0, 0, 0xff}

			toggleBtn.Importance = widget.SuccessImportance
		} else {
			toggle_mode = ASM2HEX
			app_title.Text = ApplicationTitle
			app_title.Color = color.NRGBA{0, 0x80, 0, 0xff}

			toggleBtn.Importance = widget.WarningImportance
		}
		app_title.Refresh()
		SetMode(win, status, assemblyLabel, assemblyEditor)
		triggerConversion()
		toggleBtn.Refresh()
	})
	toggleBtn.Importance = widget.WarningImportance
	clearBtn := widget.NewButtonWithIcon("Clear", theme.DeleteIcon(), func() {
		status.SetText("Clear")
		status.Refresh()
		assemblyEditor.SetText("")
		offsetInput.SetText("0")
		output1.SetText("")
	})
	clearBtn.Importance = widget.DangerImportance
	aboutBtn := widget.NewButtonWithIcon("About...", theme.QuestionIcon(), func() {
		if icons.CAPSTONE_PNG_RES != nil {
			openCapstone.Icon = icons.CAPSTONE_PNG_RES
			openCapstone.Refresh()
		}
		if icons.KEYSTONE_PNG_RES != nil {
			openKeystone.Icon = icons.KEYSTONE_PNG_RES
			openKeystone.Refresh()
		}
		status.SetText("About")
		status.Refresh()
		aboutDlg.Resize(fyne.NewSize(400, 300))
		aboutDlg.Refresh()
		aboutDlg.Show()
	})
	aboutBtn.Importance = widget.LowImportance

	status_container := container.New(layout.NewHBoxLayout(),
		status,
		layout.NewSpacer(),
		convertBtn,
		clearBtn,
		toggleBtn,
		aboutBtn,
	)
	theme.DocumentCreateIcon()

	status_container.Resize(fyne.NewSize(100, 24))

	main_layout := container.New(layout.NewVBoxLayout(),
		container.New(layout.NewHBoxLayout(),
			background,
			app_title,
			widget.NewCheck("0x", func(checked bool) {
				status.SetText("Changed")
				v := offsetInput.Text
				if checked {
					if strings.HasPrefix(v, "0x") {
						offsetInput.SetText(v)
					} else {
						offsetInput.SetText("0x" + v)
					}
					prefix_hex = true
				} else {
					if strings.HasPrefix(v, "0x") {
						offsetInput.SetText(strings.TrimPrefix(v, "0x"))
					} else {
						offsetInput.SetText(v)
					}
					prefix_hex = false
				}
				triggerConversion()
			}),
			widget.NewCheck("GDB/LLDB", func(checked bool) {
				status.SetText("Changed")
				bigEndian = checked
				if bigEndian {
					status.SetText("Big Endian")
					output1_info.SetText("Big Endian")
				} else {
					status.SetText("Little Endian")
					output1_info.SetText("Little Endian")
				}
				output1_info.Refresh()
				status.Refresh()
				triggerConversion()
			}),
			widget.NewCheck("Add Address", func(checked bool) {
				status.SetText("Add Address to output")
				addAddress = checked
				triggerConversion()
			})),
		grid,
		layout.NewSpacer(),
		status_container,
	)

	triggerConversion()

	return main_layout
}

func SetMode(win fyne.Window, status *widget.Label, assemblyLabel *widget.Label, assemblyEditor *widget.Entry) {

	if toggle_mode == ASM2HEX {
		assemblyLabel.SetText("Assembly code")
		assemblyEditor.SetPlaceHolder("Assembly code")
		status.SetText("Toggle to ASM2HEX")
		win.SetTitle(ApplicationTitle)

		asm2hexTools.Show()
		hex2asmTools.Hidden = true

	} else {
		assemblyLabel.SetText("HEX code")
		assemblyEditor.SetPlaceHolder("HEX code")
		status.SetText("Toggle to HEX2ASM")
		win.SetTitle(ApplicationTitleToggle)

		hex2asmTools.Show()
		asm2hexTools.Hidden = true
	}

	updateSelectParam()

	asm2hexTools.Refresh()
	hex2asmTools.Refresh()
	status.Refresh()
	assemblyEditor.Refresh()
	assemblyLabel.Refresh()
}

func formatEngineError(err error, marker string) string {
	if err == nil {
		return "Unknown error"
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, marker) {
		errMsg = strings.Split(errMsg, marker)[0]
	}
	return strings.TrimSpace(errMsg)
}

func doConversion(status *widget.Label,
	_output *widget.Entry,
	assemblyEditor *widget.Entry,
	offsetInput *widget.Entry) {

	// Snapshot inputs before heavy work (may run on a background goroutine).
	srcText := assemblyEditor.Text
	offsetText := offsetInput.Text
	mode := toggle_mode
	ksArch := keystone.Architecture(KSSelectParam.Arch)
	ksMode := keystone.Mode(KSSelectParam.Mode)
	ksSyntax := KSSelectParam.Syntax
	csArch := capstone.Architecture(CSSelectParam.Arch)
	csMode := capstone.Mode(CSSelectParam.Mode)
	csSyntax := CSSelectParam.Syntax
	useBigEndian := bigEndian
	useAddAddress := addAddress

	seq := convertSeq.Add(1)

	convertMu.Lock()
	defer convertMu.Unlock()

	// A newer conversion was scheduled while we waited for the lock.
	if seq != convertSeq.Load() {
		return
	}

	status.SetText("Converting...")
	status.Refresh()

	lines := strings.Split(srcText, "\n")
	curOffset, _ := strconv.ParseUint(strings.ReplaceAll(strings.ToLower(offsetText), "0x", ""), 16, 64)

	var out strings.Builder
	statusText := "Done"

	if mode == ASM2HEX {
		for _, v := range lines {
			if strings.TrimSpace(v) == "" {
				continue
			}
			encoding, _, ok, err := archs.Assemble(
				ksArch,
				ksMode,
				v,
				curOffset,
				useBigEndian,
				ksSyntax,
			)
			if !ok {
				out.WriteString(formatEngineError(err, "(KS"))
				out.WriteByte('\n')
			} else {
				if useAddAddress {
					out.WriteString(fmt.Sprintf("%08X:\t", curOffset))
				}
				out.WriteString(hexdump(encoding))
				out.WriteByte('\n')
				curOffset += uint64(len(encoding))
			}
		}
	} else {
		encoding, err := hexStringToBytes(strings.Join(lines, ""))
		if err != nil {
			if seq != convertSeq.Load() {
				return
			}
			status.SetText(err.Error())
			status.Refresh()
			_output.SetText("")
			_output.Refresh()
			return
		}
		result, _, ok, err := archs.Disassemble(
			csArch,
			csMode,
			encoding,
			curOffset,
			useBigEndian,
			csSyntax,
			useAddAddress,
		)
		if !ok {
			out.WriteString(formatEngineError(err, "(CS"))
			out.WriteByte('\n')
		} else {
			out.WriteString(result)
		}
	}

	if seq != convertSeq.Load() {
		return
	}
	status.SetText(statusText)
	status.Refresh()
	_output.SetText(out.String())
	_output.Refresh()
}

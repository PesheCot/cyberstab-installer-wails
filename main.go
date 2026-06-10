//go:build !cyberstab_uninstaller && !cyberstab_manager

package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"syscall"
	"unsafe"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	installer "cyberstab-installer/pkg/installer"
)

// showAdminRequiredAndExit shows a native Windows MessageBox and exits the process.
func showAdminRequiredAndExit() {
	const mbIconError = 0x00000010
	const mbOkOnly = 0x00000000
	const mbTopmost = 0x00040000

	title, _ := syscall.UTF16PtrFromString("Cyberstab Installer")
	message, _ := syscall.UTF16PtrFromString(
		"Для установки Cyberstab требуются права администратора.",
	)
	user32 := syscall.NewLazyDLL("user32.dll")
	msgBoxW := user32.NewProc("MessageBoxW")
	msgBoxW.Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), uintptr(mbIconError|mbOkOnly|mbTopmost))
	os.Exit(1)
}

func main() {
	// Setup file logging - use unique filename per run
	logFilePath := filepath.Join(os.TempDir(), "cyberstab-installer.log")
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// Try alternative location if temp fails
		logFilePath = filepath.Join(os.Getenv("PROGRAMDATA"), "cyberstab-installer.log")
		f, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	}

	if err == nil {
		mw := io.MultiWriter(os.Stdout, f)
		log.SetOutput(mw)
		log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
		defer f.Close()
	}

	log.Printf("=========================================")
	log.Printf("Cyberstab Installer started (PID: %d)", os.Getpid())
	log.Printf("Log file: %s", logFilePath)
	log.Printf("=========================================")

	// Windows: if started without admin rights, relaunch with UAC.
	if stdruntime.GOOS == "windows" {
		e := installer.NewEngine()
		if e.NeedSudo() && !previewInstallDoneRequested() {
			if installer.TryRelaunchAsAdmin(os.Args) {
				// UAC dialog was shown; the elevated instance is starting. Exit current.
				os.Exit(0)
			}
			// User clicked "No" in UAC dialog, or elevation failed.
			// Do NOT continue without admin rights.
			showAdminRequiredAndExit()
		}
	}

	app := NewApp()

	if runErr := wails.Run(&options.App{
		Title:         "Установщик Киберстаб",
		Width:         860,
		Height:        620,
		DisableResize: true,
		Frameless:     true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 245, G: 247, B: 251, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			DisableWindowIcon:   false,
			WebviewUserDataPath: wvDataDir(), // FIX: Edge "cannot read/write" error when running as admin
		},
	}); runErr != nil {
		log.Fatal(runErr)
	}
}

func previewInstallDoneRequested() bool {
	if os.Getenv("CYBERSTAB_PREVIEW_INSTALL_DONE") == "1" {
		return true
	}
	for _, arg := range os.Args[1:] {
		if arg == "--preview-install-done" {
			return true
		}
	}
	return false
}

//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	comdlg32          = syscall.NewLazyDLL("comdlg32.dll")
	procGetOpenFileW  = comdlg32.NewProc("GetOpenFileNameW")
	user32            = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW   = user32.NewProc("MessageBoxW")
)

type openFileNameW struct {
	lStructSize       uint32
	hwndOwner         uintptr
	hInstance         uintptr
	lpstrFilter       *uint16
	lpstrCustomFilter *uint16
	nMaxCustFilter    uint32
	nFilterIndex      uint32
	lpstrFile         *uint16
	nMaxFile          uint32
	lpstrFileTitle    *uint16
	nMaxFileTitle     uint32
	lpstrInitialDir   *uint16
	lpstrTitle        *uint16
	flags             uint32
	nFileOffset       uint16
	nFileExtension    uint16
	lpstrDefExt       *uint16
	lCustData         uintptr
	lpfnHook          uintptr
	lpTemplateName    *uint16
	pvReserved        uintptr
	dwReserved        uint32
	flagsEx           uint32
}

const (
	ofnNoChangeDir   = 0x00000008
	ofnPathMustExist = 0x00000800
	ofnFileMustExist = 0x00001000
	ofnExplorer      = 0x00080000
)

func pickExcelFileNative() (string, error) {
	filterUTF16 := encodeUTF16WithNulls("Документы Excel (*.xlsx)\x00*.xlsx\x00Все файлы (*.*)\x00*.*\x00\x00")
	titlePtr, _ := syscall.UTF16PtrFromString("Выберите файл Excel для анализа продаж")
	defExtPtr, _ := syscall.UTF16PtrFromString("xlsx")

	fileBuf := make([]uint16, 4096)
	ofn := openFileNameW{
		lStructSize: uint32(unsafe.Sizeof(openFileNameW{})),
		lpstrFilter: &filterUTF16[0],
		nFilterIndex: 1,
		lpstrFile:   &fileBuf[0],
		nMaxFile:    uint32(len(fileBuf)),
		lpstrTitle:  titlePtr,
		flags:       ofnExplorer | ofnFileMustExist | ofnPathMustExist | ofnNoChangeDir,
		lpstrDefExt: defExtPtr,
	}

	ret, _, _ := procGetOpenFileW.Call(uintptr(unsafe.Pointer(&ofn)))
	if ret == 0 {
		return "", nil
	}
	return syscall.UTF16ToString(fileBuf), nil
}

func encodeUTF16WithNulls(s string) []uint16 {
	return utf16.Encode([]rune(s))
}

func showFatalError(title, msg string) {
	tPtr, _ := syscall.UTF16PtrFromString(title)
	mPtr, _ := syscall.UTF16PtrFromString(msg)
	const mbIconError = 0x00000010
	_, _, _ = procMessageBoxW.Call(0, uintptr(unsafe.Pointer(mPtr)), uintptr(unsafe.Pointer(tPtr)), mbIconError)
}

func launchDesktopGUI(url string, onWindowExit func()) error {
	browserBin := findChromiumAppHost()
	if browserBin != "" {
		profileDir := filepath.Join(os.TempDir(), "BudgetSalesAnalyticsWebView")
		_ = os.MkdirAll(profileDir, 0o755)
		cmd := exec.Command(
			browserBin,
			"--app="+url,
			"--window-size=1440,900",
			"--user-data-dir="+profileDir,
			"--no-first-run",
			"--no-default-browser-check",
			"--disable-extensions",
		)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if err := cmd.Start(); err == nil {
			go func() {
				_ = cmd.Wait()
				if onWindowExit != nil {
					onWindowExit()
				}
			}()
			return nil
		}
	}

	// Fallback: open default Windows browser without showing console window
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}

func findChromiumAppHost() string {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("LocalAppData"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("LocalAppData"), "Google", "Chrome", "Application", "chrome.exe"),
	}
	for _, c := range candidates {
		if c != "" {
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				return c
			}
		}
	}
	for _, bin := range []string{"msedge", "chrome"} {
		if p, err := exec.LookPath(bin); err == nil {
			return p
		}
	}
	return ""
}

func hasNativePicker() bool {
	return true
}

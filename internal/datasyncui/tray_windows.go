//go:build windows

package datasyncui

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmAppTray       = 0x8000 + 1
	wmNull          = 0x0000
	wmClose         = 0x0010
	wmDestroy       = 0x0002
	wmLButtonUp     = 0x0202
	wmRButtonDown   = 0x0204
	wmRButtonUp     = 0x0205
	wmLButtonDblClk = 0x0203
	wmCommand       = 0x0111
	wmContextMenu   = 0x007b
	ninSelect       = 0x0400
	ninKeySelect    = 0x0401
	nimAdd          = 0x00000000
	nimDelete       = 0x00000002
	nimSetVersion   = 0x00000004
	nifMessage      = 0x00000001
	nifIcon         = 0x00000002
	nifTip          = 0x00000004
	nifGuid         = 0x00000020
	nifShowTip      = 0x00000080
	notifyVersion4  = 4
	idTrayIcon      = 1
	idTrayShow      = 1001
	idTrayExit      = 1002
	mfString        = 0x00000000
	mfSeparator     = 0x00000800
	tpmRightButton  = 0x00000002
	tpmReturnCmd    = 0x00000100
	idiApplication  = 32512
)

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	shell32             = windows.NewLazySystemDLL("shell32.dll")
	kernel32            = windows.NewLazySystemDLL("kernel32.dll")
	procGetModuleHandle = kernel32.NewProc("GetModuleHandleW")
	procRegisterClassEx = user32.NewProc("RegisterClassExW")
	procRegisterMessage = user32.NewProc("RegisterWindowMessageW")
	procCreateWindowEx  = user32.NewProc("CreateWindowExW")
	procCreateIcon      = user32.NewProc("CreateIcon")
	procDefWindowProc   = user32.NewProc("DefWindowProcW")
	procDestroyWindow   = user32.NewProc("DestroyWindow")
	procDestroyIcon     = user32.NewProc("DestroyIcon")
	procGetMessage      = user32.NewProc("GetMessageW")
	procTranslateMsg    = user32.NewProc("TranslateMessage")
	procDispatchMsg     = user32.NewProc("DispatchMessageW")
	procPostQuitMessage = user32.NewProc("PostQuitMessage")
	procPostMessage     = user32.NewProc("PostMessageW")
	procLoadIcon        = user32.NewProc("LoadIconW")
	procCreatePopupMenu = user32.NewProc("CreatePopupMenu")
	procAppendMenu      = user32.NewProc("AppendMenuW")
	procTrackPopupMenu  = user32.NewProc("TrackPopupMenu")
	procDestroyMenu     = user32.NewProc("DestroyMenu")
	procSetForeground   = user32.NewProc("SetForegroundWindow")
	procGetCursorPos    = user32.NewProc("GetCursorPos")
	procShellNotifyIcon = shell32.NewProc("Shell_NotifyIconW")
)

type nativeTrayCallbacks struct {
	Show        func()
	RequestExit func()
}

type nativeTray struct {
	callbacks nativeTrayCallbacks
	ready     chan error
	done      chan struct{}
	hwnd      windows.Handle
	icon      windows.Handle
	taskbar   uint32
	ownIcon   bool
	stopOnce  sync.Once
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     windows.Handle
	hIcon         windows.Handle
	hCursor       windows.Handle
	hbrBackground windows.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       windows.Handle
}

type point struct {
	x int32
	y int32
}

type msg struct {
	hwnd    windows.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type notifyIconData struct {
	cbSize           uint32
	hWnd             windows.Handle
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            windows.Handle
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         windows.GUID
	hBalloonIcon     windows.Handle
}

func startNativeTray(callbacks nativeTrayCallbacks) (*nativeTray, error) {
	tray := &nativeTray{
		callbacks: callbacks,
		ready:     make(chan error, 1),
		done:      make(chan struct{}),
	}
	go tray.run()
	if err := <-tray.ready; err != nil {
		return nil, err
	}
	return tray, nil
}

func (t *nativeTray) Stop() {
	t.stopOnce.Do(func() {
		if t.hwnd != 0 {
			procPostMessage.Call(uintptr(t.hwnd), wmClose, 0, 0)
		}
		<-t.done
	})
}

func (t *nativeTray) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(t.done)

	instance, _, _ := procGetModuleHandle.Call(0)
	className, _ := windows.UTF16PtrFromString("NodeBridgeDataSyncTrayWindow")
	taskbarCreated, _ := windows.UTF16PtrFromString("TaskbarCreated")
	taskbarMessage, _, _ := procRegisterMessage.Call(uintptr(unsafe.Pointer(taskbarCreated)))
	t.taskbar = uint32(taskbarMessage)
	wndProc := syscall.NewCallback(t.windowProc)
	class := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   wndProc,
		hInstance:     windows.Handle(instance),
		lpszClassName: className,
	}
	if r, _, callErr := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&class))); r == 0 {
		t.ready <- callErr
		return
	}

	hwnd, _, callErr := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(className)),
		0,
		0, 0, 0, 0,
		0, 0,
		instance,
		0,
	)
	if hwnd == 0 {
		t.ready <- callErr
		return
	}
	t.hwnd = windows.Handle(hwnd)

	icon, err := createTrayIcon(windows.Handle(instance))
	if err != nil {
		icon, _, _ := procLoadIcon.Call(0, uintptr(idiApplication))
		t.icon = windows.Handle(icon)
	} else {
		t.icon = icon
		t.ownIcon = true
	}
	if err := t.addIcon(); err != nil {
		t.ready <- err
		return
	}
	t.ready <- nil

	var message msg
	for {
		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(ret) <= 0 {
			return
		}
		procTranslateMsg.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMsg.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func (t *nativeTray) windowProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case wmAppTray:
		switch uint32(lParam & 0xffff) {
		case wmLButtonUp, wmLButtonDblClk, ninSelect, ninKeySelect:
			t.showWindow()
		case wmRButtonDown, wmRButtonUp, wmContextMenu:
			t.showMenu()
		}
		return 0
	case t.taskbar:
		_ = t.addIcon()
		return 0
	case wmCommand:
		switch uint32(wParam & 0xffff) {
		case idTrayShow:
			t.showWindow()
		case idTrayExit:
			t.requestExit()
		}
		return 0
	case wmClose:
		t.deleteIcon()
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	default:
		ret, _, _ := procDefWindowProc.Call(hwnd, uintptr(message), wParam, lParam)
		return ret
	}
}

func (t *nativeTray) addIcon() error {
	data := t.iconData()
	if r, _, callErr := procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&data))); r == 0 {
		return fmt.Errorf("Shell_NotifyIcon add failed: %w", callErr)
	}
	data.uVersion = notifyVersion4
	if r, _, callErr := procShellNotifyIcon.Call(nimSetVersion, uintptr(unsafe.Pointer(&data))); r == 0 {
		log.Printf("Shell_NotifyIcon set version failed: %v", callErr)
	}
	log.Print("native tray icon registered")
	return nil
}

func (t *nativeTray) deleteIcon() {
	data := t.iconData()
	if r, _, callErr := procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&data))); r == 0 {
		log.Printf("Shell_NotifyIcon delete failed: %v", callErr)
	}
	if t.ownIcon && t.icon != 0 {
		procDestroyIcon.Call(uintptr(t.icon))
		t.icon = 0
	}
}

func (t *nativeTray) iconData() notifyIconData {
	var data notifyIconData
	data.cbSize = uint32(unsafe.Sizeof(data))
	data.hWnd = t.hwnd
	data.uID = idTrayIcon
	data.uFlags = nifMessage | nifIcon | nifTip | nifGuid | nifShowTip
	data.uCallbackMessage = wmAppTray
	data.hIcon = t.icon
	data.guidItem = windows.GUID{
		Data1: 0x3a85c0de,
		Data2: 0x5d7a,
		Data3: 0x4a13,
		Data4: [8]byte{0xa9, 0x68, 0x4e, 0x6f, 0x64, 0x65, 0x42, 0x72},
	}
	copy(data.szTip[:], windows.StringToUTF16("NodeBridge"))
	return data
}

func createTrayIcon(instance windows.Handle) (windows.Handle, error) {
	const size = 32
	andMask := make([]byte, size*size/8)
	xorMask := make([]byte, size*size*4)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			offset := (y*size + x) * 4
			xorMask[offset+0] = 0x22
			xorMask[offset+1] = 0xd3
			xorMask[offset+2] = 0xee
			xorMask[offset+3] = 0xff
			if x < 4 || x >= size-4 || y < 4 || y >= size-4 {
				xorMask[offset+0] = 0x08
				xorMask[offset+1] = 0x1a
				xorMask[offset+2] = 0x1f
			}
		}
	}
	icon, _, callErr := procCreateIcon.Call(
		uintptr(instance),
		size,
		size,
		1,
		32,
		uintptr(unsafe.Pointer(&andMask[0])),
		uintptr(unsafe.Pointer(&xorMask[0])),
	)
	if icon == 0 {
		return 0, fmt.Errorf("CreateIcon failed: %w", callErr)
	}
	return windows.Handle(icon), nil
}

func (t *nativeTray) showMenu() {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	showText, _ := windows.UTF16PtrFromString("显示 NodeBridge")
	exitText, _ := windows.UTF16PtrFromString("退出...")
	procAppendMenu.Call(menu, mfString, idTrayShow, uintptr(unsafe.Pointer(showText)))
	procAppendMenu.Call(menu, mfSeparator, 0, 0)
	procAppendMenu.Call(menu, mfString, idTrayExit, uintptr(unsafe.Pointer(exitText)))

	var cursor point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))
	procSetForeground.Call(uintptr(t.hwnd))
	cmd, _, _ := procTrackPopupMenu.Call(menu, tpmRightButton|tpmReturnCmd, uintptr(cursor.x), uintptr(cursor.y), 0, uintptr(t.hwnd), 0)
	procPostMessage.Call(uintptr(t.hwnd), wmNull, 0, 0)
	switch uint32(cmd) {
	case idTrayShow:
		t.showWindow()
	case idTrayExit:
		t.requestExit()
	}
}

func (t *nativeTray) showWindow() {
	if t.callbacks.Show != nil {
		go t.callbacks.Show()
	}
}

func (t *nativeTray) requestExit() {
	if t.callbacks.RequestExit != nil {
		go t.callbacks.RequestExit()
	}
}

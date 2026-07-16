//go:build windows

package backend

import (
	"ant-chrome/backend/internal/iconbadge"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	imageIcon      = 1
	lrLoadFromFile = 0x0010
	vtLPWSTR       = 31
)

var (
	profileTaskbarUser32                 = windows.NewLazySystemDLL("user32.dll")
	profileTaskbarShell32                = windows.NewLazySystemDLL("shell32.dll")
	profileTaskbarOle32                  = windows.NewLazySystemDLL("ole32.dll")
	procProfileTaskbarEnumWindows        = profileTaskbarUser32.NewProc("EnumWindows")
	procProfileTaskbarGetWindowThreadPID = profileTaskbarUser32.NewProc("GetWindowThreadProcessId")
	procProfileTaskbarIsWindowVisible    = profileTaskbarUser32.NewProc("IsWindowVisible")
	procProfileTaskbarLoadImage          = profileTaskbarUser32.NewProc("LoadImageW")
	procProfileTaskbarDestroyIcon        = profileTaskbarUser32.NewProc("DestroyIcon")
	procSHGetPropertyStoreForWindow      = profileTaskbarShell32.NewProc("SHGetPropertyStoreForWindow")
	procProfileTaskbarCoCreateInstance   = profileTaskbarOle32.NewProc("CoCreateInstance")
	clsidTaskbarList                     = windows.GUID{Data1: 0x56FDF344, Data2: 0xFD6D, Data3: 0x11D0, Data4: [8]byte{0x95, 0x8A, 0x00, 0x60, 0x97, 0xC9, 0xA0, 0x90}}
	iidTaskbarList3                      = windows.GUID{Data1: 0xEA1AFB91, Data2: 0x9E28, Data3: 0x4B86, Data4: [8]byte{0x90, 0xE9, 0x9E, 0x9F, 0x8A, 0x5E, 0xEA, 0x84}}
	iidPropertyStore                     = windows.GUID{Data1: 0x886D8EEB, Data2: 0x8CF2, Data3: 0x4446, Data4: [8]byte{0x8D, 0x02, 0xCD, 0xBA, 0x1D, 0xBD, 0xCF, 0x99}}
	appUserModelIDKey                    = propertyKey{FormatID: windows.GUID{Data1: 0x9F4C2855, Data2: 0x9F79, Data3: 0x4B39, Data4: [8]byte{0xA8, 0xD0, 0xE1, 0xD4, 0x2D, 0xE1, 0xD5, 0xF3}}, PropertyID: 5}
)

type propertyKey struct {
	FormatID   windows.GUID
	PropertyID uint32
}

type propVariant struct {
	ValueType uint16
	Reserved1 uint16
	Reserved2 uint16
	Reserved3 uint16
	Value     uintptr
	Value2    uintptr
}

type unknownVTable struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
}

type propertyStoreVTable struct {
	unknownVTable
	GetCount uintptr
	GetAt    uintptr
	GetValue uintptr
	SetValue uintptr
	Commit   uintptr
}

type propertyStore struct {
	VTable *propertyStoreVTable
}

type taskbarList3VTable struct {
	unknownVTable
	HrInit                uintptr
	AddTab                uintptr
	DeleteTab             uintptr
	ActivateTab           uintptr
	SetActiveAlt          uintptr
	MarkFullscreenWindow  uintptr
	SetProgressValue      uintptr
	SetProgressState      uintptr
	RegisterTab           uintptr
	UnregisterTab         uintptr
	SetTabOrder           uintptr
	SetTabActive          uintptr
	ThumbBarAddButtons    uintptr
	ThumbBarUpdateButtons uintptr
	ThumbBarSetImageList  uintptr
	SetOverlayIcon        uintptr
}

type taskbarList3 struct {
	VTable *taskbarList3VTable
}

func applyPlatformProfileBrowserTaskbarIcon(stateRoot string, profile *BrowserProfile, processID int) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	initialized := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED) == nil
	if initialized {
		defer windows.CoUninitialize()
	}

	hwnd := findProfileBrowserWindow(uint32(processID), 3*time.Second)
	if hwnd == 0 {
		return fmt.Errorf("未找到实例浏览器窗口")
	}
	appID := fmt.Sprintf("HiBrowser.Profile.%x", sha256.Sum256([]byte(profile.ProfileId)))[:34]
	if err := setWindowAppUserModelID(hwnd, appID); err != nil {
		return err
	}
	icoPath := filepath.Join(stateRoot, "data", "cache", "profile-browser-icons", profile.ProfileId, "taskbar.ico")
	if err := writeTaskbarOverlayICO(icoPath, profile.IconBadge, profile.IconBadgeColor); err != nil {
		return err
	}
	iconPath, err := windows.UTF16PtrFromString(icoPath)
	if err != nil {
		return fmt.Errorf("解析任务栏图标路径失败: %w", err)
	}
	hicon, _, loadErr := procProfileTaskbarLoadImage.Call(0, uintptr(unsafe.Pointer(iconPath)), imageIcon, 0, 0, lrLoadFromFile)
	if hicon == 0 {
		return fmt.Errorf("加载任务栏角标失败: %w", loadErr)
	}
	defer procProfileTaskbarDestroyIcon.Call(hicon)

	var taskbar *taskbarList3
	hr, _, _ := procProfileTaskbarCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidTaskbarList)),
		0,
		windows.CLSCTX_INPROC_SERVER,
		uintptr(unsafe.Pointer(&iidTaskbarList3)),
		uintptr(unsafe.Pointer(&taskbar)),
	)
	if hresultFailed(hr) || taskbar == nil {
		return fmt.Errorf("创建 Windows 任务栏接口失败: 0x%08X", uint32(hr))
	}
	defer syscall.SyscallN(taskbar.VTable.Release, uintptr(unsafe.Pointer(taskbar)))
	if hr, _, _ := syscall.SyscallN(taskbar.VTable.HrInit, uintptr(unsafe.Pointer(taskbar))); hresultFailed(hr) {
		return fmt.Errorf("初始化 Windows 任务栏接口失败: 0x%08X", uint32(hr))
	}
	description, _ := windows.UTF16PtrFromString("Hi Browser " + profile.IconBadge)
	hr, _, _ = syscall.SyscallN(taskbar.VTable.SetOverlayIcon, uintptr(unsafe.Pointer(taskbar)), hwnd, hicon, uintptr(unsafe.Pointer(description)))
	if hresultFailed(hr) {
		return fmt.Errorf("设置 Windows 任务栏角标失败: 0x%08X", uint32(hr))
	}
	return nil
}

func findProfileBrowserWindow(processID uint32, timeout time.Duration) uintptr {
	deadline := time.Now().Add(timeout)
	for {
		var found uintptr
		callback := windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
			var windowProcessID uint32
			procProfileTaskbarGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&windowProcessID)))
			visible, _, _ := procProfileTaskbarIsWindowVisible.Call(hwnd)
			if windowProcessID == processID && visible != 0 {
				found = hwnd
				return 0
			}
			return 1
		})
		procProfileTaskbarEnumWindows.Call(callback, 0)
		if found != 0 || time.Now().After(deadline) {
			return found
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func setWindowAppUserModelID(hwnd uintptr, appID string) error {
	var store *propertyStore
	hr, _, _ := procSHGetPropertyStoreForWindow.Call(hwnd, uintptr(unsafe.Pointer(&iidPropertyStore)), uintptr(unsafe.Pointer(&store)))
	if hresultFailed(hr) || store == nil {
		return fmt.Errorf("读取窗口任务栏标识失败: 0x%08X", uint32(hr))
	}
	defer syscall.SyscallN(store.VTable.Release, uintptr(unsafe.Pointer(store)))
	value, err := windows.UTF16PtrFromString(appID)
	if err != nil {
		return fmt.Errorf("生成窗口任务栏标识失败: %w", err)
	}
	variant := propVariant{ValueType: vtLPWSTR, Value: uintptr(unsafe.Pointer(value))}
	hr, _, _ = syscall.SyscallN(store.VTable.SetValue, uintptr(unsafe.Pointer(store)), uintptr(unsafe.Pointer(&appUserModelIDKey)), uintptr(unsafe.Pointer(&variant)))
	if hresultFailed(hr) {
		return fmt.Errorf("设置窗口任务栏标识失败: 0x%08X", uint32(hr))
	}
	hr, _, _ = syscall.SyscallN(store.VTable.Commit, uintptr(unsafe.Pointer(store)))
	if hresultFailed(hr) {
		return fmt.Errorf("提交窗口任务栏标识失败: 0x%08X", uint32(hr))
	}
	return nil
}

func writeTaskbarOverlayICO(path, badge, badgeColor string) error {
	img, err := iconbadge.Render(image.NewNRGBA(image.Rect(0, 0, 64, 64)), 64, badge, badgeColor)
	if err != nil {
		return fmt.Errorf("绘制任务栏角标失败: %w", err)
	}
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, img); err != nil {
		return fmt.Errorf("编码任务栏角标失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建任务栏角标目录失败: %w", err)
	}
	var ico bytes.Buffer
	_ = binary.Write(&ico, binary.LittleEndian, uint16(0))
	_ = binary.Write(&ico, binary.LittleEndian, uint16(1))
	_ = binary.Write(&ico, binary.LittleEndian, uint16(1))
	ico.Write([]byte{64, 64, 0, 0})
	_ = binary.Write(&ico, binary.LittleEndian, uint16(1))
	_ = binary.Write(&ico, binary.LittleEndian, uint16(32))
	_ = binary.Write(&ico, binary.LittleEndian, uint32(pngData.Len()))
	_ = binary.Write(&ico, binary.LittleEndian, uint32(22))
	ico.Write(pngData.Bytes())
	if err := os.WriteFile(path, ico.Bytes(), 0o644); err != nil {
		return fmt.Errorf("保存任务栏角标失败: %w", err)
	}
	return nil
}

func hresultFailed(result uintptr) bool {
	return int32(result) < 0
}

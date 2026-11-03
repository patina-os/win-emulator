// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// hostDefaults is what Windows says: its language, keyboard layout, time zone and user.
func hostDefaults() setupDefaults {
	locale := localeFor(userLocaleName())
	d := setupDefaults{Locale: locale, Keyboard: keyboardFor(keyboardLayoutName()), TimeZone: hostTimeZone(),
		German: strings.HasPrefix(locale, "de_"), Locales: localeChoices(locale), Keyboards: keyboardChoices(),
		TimeZones: timeZones}
	if u, err := user.Current(); err == nil {
		d.User = loginName(u.Username)
		d.FullName = u.Name
	}
	return d
}

// userLocaleName is the user's locale, such as de-DE.
func userLocaleName() string {
	buf := make([]uint16, 85) // LOCALE_NAME_MAX_LENGTH
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")
	if n, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))); n == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

// keyboardLayoutName is the keyboard layout ID (KLID) of the user's input language,
// such as 00000407.
func keyboardLayoutName() string {
	buf := make([]uint16, 9) // KL_NAMELENGTH
	proc := windows.NewLazySystemDLL("user32.dll").NewProc("GetKeyboardLayoutNameW")
	if ok, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0]))); ok == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}

// hostTimeZone is Windows' time zone as an IANA name, such as Europe/Berlin: ICU, which
// Windows ships from 10 1903 on, maps every Windows zone; older ones get the zones of
// Europe and North America; UTC where neither knows it.
func hostTimeZone() string {
	var tz struct { // DYNAMIC_TIME_ZONE_INFORMATION
		Bias         int32
		StandardName [32]uint16
		StandardDate windows.Systemtime
		StandardBias int32
		DaylightName [32]uint16
		DaylightDate windows.Systemtime
		DaylightBias int32
		KeyName      [128]uint16
		NoDaylight   uint8
	}
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetDynamicTimeZoneInformation")
	if r, _, _ := proc.Call(uintptr(unsafe.Pointer(&tz))); r == 0xFFFFFFFF { // TIME_ZONE_ID_INVALID
		return "Etc/UTC"
	}
	key := windows.UTF16ToString(tz.KeyName[:])
	if icu := windows.NewLazySystemDLL("icu.dll").NewProc("ucal_getTimeZoneIDForWindowsID"); icu.Find() == nil {
		winID, _ := windows.UTF16FromString(key)
		id := make([]uint16, 64)
		var status int32 // UErrorCode; above zero is a failure
		n, _, _ := icu.Call(uintptr(unsafe.Pointer(&winID[0])), uintptr(len(winID)-1), 0,
			uintptr(unsafe.Pointer(&id[0])), uintptr(len(id)), uintptr(unsafe.Pointer(&status)))
		if status <= 0 && int32(n) > 0 && int(n) < len(id) {
			return windows.UTF16ToString(id[:n])
		}
	}
	if z, ok := timeZoneFallback[key]; ok {
		return z
	}
	return "Etc/UTC"
}

// edgePath is Microsoft Edge's program, empty where it is not installed (it can be
// removed in the European Economic Area).
func edgePath() string {
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		k, err := registry.OpenKey(root, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\msedge.exe`, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		p, _, err := k.GetStringValue("")
		k.Close()
		if err == nil && fileExists(p) {
			return p
		}
	}
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "LOCALAPPDATA"} {
		if p := filepath.Join(os.Getenv(env), `Microsoft\Edge\Application\msedge.exe`); fileExists(p) {
			return p
		}
	}
	return ""
}

// askSetup shows the setup page in a window of its own: Edge as an app (the engine of
// Windows' WebView2), with a profile of its own in the data directory, so it neither
// joins a running Edge nor keeps anything. Without Edge the default browser shows it.
func (a *app) askSetup(p setupPages) (setupAnswers, error) {
	icon, _ := assets.ReadFile("assets/icon-small-64.png")
	s, err := startSetup(p, hostDefaults(), icon)
	if err != nil {
		return setupAnswers{}, err
	}
	defer s.Close()
	profile := filepath.Join(a.dir, "setup-window")
	var edge *exec.Cmd
	closed := make(chan struct{})
	if path := edgePath(); path != "" {
		edge = exec.Command(path, "--app="+s.URL, "--user-data-dir="+profile, "--no-first-run",
			"--no-default-browser-check", "--disable-sync", "--window-size=1040,820")
		if err := edge.Start(); err != nil {
			a.log.Printf("Edge does not start: %v", err)
			edge = nil
		}
	}
	if edge != nil {
		go func() {
			started := time.Now()
			edge.Wait()
			// An Edge that ends at once handed the window to another one; then the page's
			// own signs of life tell when it closes.
			if time.Since(started) > 5*time.Second {
				close(closed)
			}
		}()
	} else {
		a.log.Print("no Edge: the setup opens in the default browser")
		open(s.URL)
	}
	answers, err := s.wait(closed, 2*time.Minute)
	if edge != nil {
		time.Sleep(1500 * time.Millisecond) // the page says it is done
		edge.Process.Kill()
	}
	time.AfterFunc(5*time.Second, func() { os.RemoveAll(profile) }) // once Edge's helpers ended
	return answers, err
}

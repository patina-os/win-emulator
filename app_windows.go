// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"fyne.io/systray"
	"github.com/ncruces/zenity"
)

func main() {
	if !singleInstance() {
		zenity.Info(appName+" is already running: its window and its icon in the notification area are open.",
			zenity.Title(appName))
		return
	}
	a, err := newApp()
	if err != nil {
		zenity.Error(err.Error(), zenity.Title(appName))
		os.Exit(1)
	}
	systray.Run(a.onReady, nil)
}

type app struct {
	dir     string // %LOCALAPPDATA%\patina-vm
	icon    string // patina.ico in dir, for the tray and the dialogs
	log     *log.Logger
	logFile *os.File
	http    *http.Client
	qemuDir string
	accel   string

	mu    sync.Mutex
	qmp   *QMP   // the running machine, nil while none runs
	after string // what follows when the machine has stopped: "", "reset" or "remove"

	status, shutdown, forceOff, folder, reset, remove, update, about *systray.MenuItem
}

func newApp() (*app, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		var err error
		if base, err = os.UserCacheDir(); err != nil {
			return nil, err
		}
	}
	a := &app{dir: filepath.Join(base, "patina-vm"), http: &http.Client{}, accel: accelWHPX}
	if err := os.MkdirAll(a.dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(a.dir, appLog), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	a.logFile = f
	a.log = log.New(f, "", log.LstdFlags)
	a.log.Printf("Try patina %s starts, data in %s", version, a.dir)
	a.icon = filepath.Join(a.dir, iconFile)
	if ico, err := appIcon(); err == nil {
		os.WriteFile(a.icon, ico, 0o644)
	}
	return a, nil
}

// Dialog options every dialog shares.
func (a *app) opts(more ...zenity.Option) []zenity.Option {
	o := []zenity.Option{zenity.Title(appName)}
	if fileExists(a.icon) {
		o = append(o, zenity.WindowIcon(a.icon))
	}
	return append(o, more...)
}

func (a *app) onReady() {
	if fileExists(a.icon) {
		systray.SetIconFromFilePath(a.icon)
	}
	systray.SetTooltip(appName)
	a.status = systray.AddMenuItem(appName+": setting up", "")
	a.status.Disable()
	systray.AddSeparator()
	a.shutdown = systray.AddMenuItem("Shut down patina", "Shuts patina down as its own menu would")
	a.forceOff = systray.AddMenuItem("Turn off now…", "Turns the virtual machine off at once")
	systray.AddSeparator()
	a.folder = systray.AddMenuItem("Open the data folder", "The virtual disk and the logs")
	a.reset = systray.AddMenuItem("Install patina again…", "Deletes the virtual disk and installs patina anew")
	a.remove = systray.AddMenuItem("Remove patina from this PC…", "Deletes the virtual disk and the data folder")
	systray.AddSeparator()
	a.update = systray.AddMenuItem("", "")
	a.update.Hide()
	a.about = systray.AddMenuItem("About Try patina", "")
	a.running(nil)
	go a.menu()
	go a.checkUpdate()
	go func() {
		if err := a.run(); err != nil && !errors.Is(err, zenity.ErrCanceled) {
			a.log.Printf("error: %v", err)
			zenity.Error(err.Error()+"\n\nThe log is in "+a.dir+".", a.opts()...)
		}
		a.log.Print("Try patina quits")
		a.logFile.Close()
		systray.Quit()
	}()
}

// running records the machine that runs (nil: none) and fits the menu to it.
func (a *app) running(q *QMP) {
	a.mu.Lock()
	a.qmp = q
	a.mu.Unlock()
	for _, m := range []*systray.MenuItem{a.shutdown, a.forceOff, a.reset, a.remove} {
		if q != nil {
			m.Enable()
		} else {
			m.Disable()
		}
	}
	if q != nil {
		a.status.SetTitle(appName + ": patina is running")
	} else {
		a.status.SetTitle(appName + ": patina is not running")
	}
}

// command sends a QMP command to the running machine, if one runs.
func (a *app) command(c string) {
	a.mu.Lock()
	q := a.qmp
	a.mu.Unlock()
	if q == nil {
		return
	}
	if err := q.Execute(c); err != nil {
		a.log.Printf("%s: %v", c, err)
	}
}

func (a *app) menu() {
	for {
		select {
		case <-a.shutdown.ClickedCh:
			a.command("system_powerdown")
		case <-a.forceOff.ClickedCh:
			if zenity.Question("Turn the virtual machine off now? Work that is not saved in patina is lost.",
				a.opts(zenity.OKLabel("Turn off"), zenity.CancelLabel("Keep running"), zenity.WarningIcon)...) == nil {
				a.command("quit")
			}
		case <-a.folder.ClickedCh:
			open(a.dir)
		case <-a.reset.ClickedCh:
			if zenity.Question("Delete patina's virtual disk and install patina again? Everything in it is lost: "+
				"files, apps and settings. Windows is not touched.",
				a.opts(zenity.OKLabel("Delete and install"), zenity.CancelLabel("Keep patina"), zenity.WarningIcon)...) == nil {
				a.stopFor("reset")
			}
		case <-a.remove.ClickedCh:
			if zenity.Question("Remove patina from this PC? This deletes the virtual disk with everything in it, and "+
				"the data folder "+a.dir+". QEMU stays installed; remove it under Settings, Apps.",
				a.opts(zenity.OKLabel("Remove"), zenity.CancelLabel("Keep patina"), zenity.WarningIcon)...) == nil {
				a.stopFor("remove")
			}
		case <-a.update.ClickedCh:
			open(github + "/" + launcherRepo + "/releases/latest")
		case <-a.about.ClickedCh:
			zenity.Info(fmt.Sprintf("Try patina %s\n\npatina in a virtual machine on Windows: stock QEMU on the "+
				"Windows Hypervisor Platform, and patina from its released image.\n\n"+
				"github.com/%s\ngithub.com/%s\n\nGPL-3.0-or-later", version, launcherRepo, patinaRepo), a.opts()...)
		}
	}
}

// stopFor turns the machine off and leaves what follows to run.
func (a *app) stopFor(after string) {
	a.mu.Lock()
	a.after = after
	a.mu.Unlock()
	a.command("quit")
}

// checkUpdate shows a menu entry when a newer launcher is released.
func (a *app) checkUpdate() {
	if version == "dev" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tag, err := latestRelease(ctx, a.http, launcherRepo)
	if err != nil || tag == version {
		return
	}
	a.update.SetTitle("Try patina " + tag + " is available…")
	a.update.Show()
}

// run is the launcher's life: set patina up until it is installed, then start it, and
// do what the menu asked for once it has stopped.
func (a *app) run() error {
	for {
		st, err := loadState(a.dir)
		if err != nil {
			return err
		}
		if st.Phase != phaseInstalled {
			if done, err := a.setup(st); err != nil || !done {
				return err
			}
			continue
		}
		if a.qemuDir == "" {
			if err := a.prepare(); err != nil {
				return err
			}
		}
		if _, err := a.boot(""); err != nil {
			return err
		}
		a.mu.Lock()
		after := a.after
		a.after = ""
		a.mu.Unlock()
		switch after {
		case "reset":
			if err := a.wipe(); err != nil {
				return err
			}
		case "remove":
			return a.removeAll()
		default:
			return nil
		}
	}
}

// removeAll deletes the data folder; the launcher quits after it.
func (a *app) removeAll() error {
	a.log.Print("removing the data folder")
	a.logFile.Close()
	if err := os.RemoveAll(a.dir); err != nil {
		return fmt.Errorf("could not delete %s: %w", a.dir, err)
	}
	zenity.Info("patina is removed. Delete TryPatina.exe to finish.", a.opts()...)
	return zenity.ErrCanceled
}

// wipe deletes the machine but keeps the logs, so the next run sets up anew.
func (a *app) wipe() error {
	for _, f := range []string{diskFile, varsFile, stateFile} {
		if err := os.Remove(filepath.Join(a.dir, f)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	a.log.Print("the machine is deleted")
	return nil
}

// prepare checks the host and finds QEMU, installing it when the user agrees.
func (a *app) prepare() error {
	if armHost() {
		return errors.New("Try patina needs an x86_64 PC: Windows on ARM cannot run patina's x86_64 image in its hypervisor yet")
	}
	if !hypervisorPresent() {
		err := zenity.Question("The Windows Hypervisor Platform is off, so patina would run very slowly.\n\n"+
			"Turn it on in \"Turn Windows features on or off\": tick \"Windows Hypervisor Platform\", "+
			"press OK, restart Windows, and open Try patina again. Virtualization must also be on in "+
			"the PC's firmware settings (Intel VT-x or AMD-V).",
			a.opts(zenity.OKLabel("Open Windows features"), zenity.ExtraButton("Continue slowly"),
				zenity.CancelLabel("Quit"), zenity.WarningIcon)...)
		switch {
		case err == nil:
			open("optionalfeatures.exe")
			return zenity.ErrCanceled
		case errors.Is(err, zenity.ErrExtraButton):
			a.accel = accelEmulation
		default:
			return zenity.ErrCanceled
		}
	}
	a.qemuDir = findQEMU()
	if a.qemuDir != "" {
		return nil
	}
	if err := zenity.Question("Try patina runs patina in QEMU, the free virtual machine. It is not installed yet.\n\n"+
		"Install it now with winget? Windows asks for permission.",
		a.opts(zenity.OKLabel("Install QEMU"), zenity.CancelLabel("Quit"), zenity.QuestionIcon)...); err != nil {
		return zenity.ErrCanceled
	}
	dlg, _ := zenity.Progress(a.opts(zenity.Pulsate(), zenity.NoCancel())...)
	if dlg != nil {
		dlg.Text("Installing QEMU with winget…")
	}
	err := installQEMU()
	if dlg != nil {
		dlg.Close()
	}
	if err != nil {
		return err
	}
	if a.qemuDir = findQEMU(); a.qemuDir == "" {
		return errors.New("QEMU was installed, but its files are not under Program Files\\qemu")
	}
	a.log.Printf("QEMU in %s", a.qemuDir)
	return nil
}

// setup takes a machine from nothing (or an unfinished install) to patina installed on
// its disk. done is false when the user stopped it; it then says how to go on.
func (a *app) setup(st State) (done bool, err error) {
	if st.Phase == phaseNew {
		err = zenity.Question("Try patina runs patina in a window, next to Windows, without changing your "+
			"drive.\n\nThe setup installs QEMU if it is missing, downloads patina's installer (about 1 GB) "+
			"and runs it on a virtual disk. patina then sets up its apps on its first start, which "+
			"takes a while and needs the internet.\n\n"+
			fmt.Sprintf("Keep about %d GB free on this drive: the virtual disk grows as patina uses it.", minFreeSpace>>30),
			a.opts(zenity.OKLabel("Set up patina"), zenity.CancelLabel("Not now"), zenity.QuestionIcon,
				zenity.Width(460))...)
	} else {
		// The launcher saw no restart by the installer, but the installer may have
		// finished all the same (it was closed, or the restart went unnoticed), so the
		// user can say so and start the disk.
		const (
			again     = "Start the installer again"
			installed = "patina is installed: start it"
			over      = "Start over with an empty disk"
		)
		var choice string
		choice, err = zenity.List("The installation of patina did not finish last time, as far as Try patina "+
			"can tell. What now?", []string{again, installed, over},
			a.opts(zenity.DefaultItems(again), zenity.OKLabel("Continue"), zenity.CancelLabel("Not now"),
				zenity.Width(480), zenity.Height(320))...)
		switch {
		case err != nil:
		case choice == installed:
			if fi, serr := os.Stat(filepath.Join(a.dir, diskFile)); serr == nil && fi.Size() >= minInstalledDisk {
				st.Phase = phaseInstalled
				a.log.Print("the user says patina is installed")
				return true, st.save(a.dir)
			}
			zenity.Info("The virtual disk is almost empty, so patina is not on it yet. The installer opens instead.",
				a.opts()...)
		case choice == over:
			if err := a.wipe(); err != nil {
				return false, err
			}
			st = State{}
		}
	}
	if err != nil {
		return false, nil
	}
	if err := a.prepare(); err != nil {
		return false, err
	}
	if free := freeBytes(a.dir); free > 0 && free < minFreeSpace {
		if zenity.Question(fmt.Sprintf("The drive of %s has %d GB free; patina needs about %d GB. "+
			"Continue anyway?", a.dir, free>>30, minFreeSpace>>30),
			a.opts(zenity.OKLabel("Continue"), zenity.CancelLabel("Quit"), zenity.WarningIcon)...) != nil {
			return false, nil
		}
	}

	iso, tag, err := a.fetchImage()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return false, nil
		}
		return false, err
	}
	if st.Phase == phaseNew || !fileExists(filepath.Join(a.dir, diskFile)) {
		if err := a.createDisk(); err != nil {
			return false, err
		}
	}
	st = State{Phase: phaseInstalling, Release: tag}
	if err := st.save(a.dir); err != nil {
		return false, err
	}
	zenity.Info("patina's installer opens in a window now.\n\n"+
		"• Pick your language, keyboard and account as on a PC. On the disk page, \"Erase disk\" erases "+
		"only the virtual disk; Windows is not touched.\n"+
		"• When the installer is done, patina restarts by itself and sets up its apps.\n"+
		"• The window's View menu has full screen and zoom. To close patina, shut it down from its "+
		"own menu or from Try patina's icon in the notification area.",
		a.opts(zenity.OKLabel("Open the installer"), zenity.InfoIcon, zenity.Width(460))...)

	reason, err := a.boot(iso)
	if err != nil {
		return false, err
	}
	size := int64(0)
	if fi, err := os.Stat(filepath.Join(a.dir, diskFile)); err == nil {
		size = fi.Size()
	}
	installed := false
	switch {
	case size < minInstalledDisk:
	case reason == "guest-reset":
		installed = true // the installer's last page restarts the machine
	case reason == "guest-shutdown":
		installed = zenity.Question("patina was shut down. Did the installer finish?",
			a.opts(zenity.OKLabel("Yes, start patina"), zenity.CancelLabel("No"), zenity.QuestionIcon)...) == nil
	}
	a.mu.Lock()
	after := a.after
	a.after = ""
	a.mu.Unlock()
	switch after {
	case "reset":
		if err := a.wipe(); err != nil {
			return false, err
		}
		return a.setup(State{})
	case "remove":
		return false, a.removeAll()
	}
	if !installed {
		a.log.Printf("the install run ended (%s, disk %d MB) before the installer finished", reason, size>>20)
		zenity.Info("The installation stopped before it finished. Open Try patina again to continue.", a.opts()...)
		return false, nil
	}
	a.log.Printf("patina %s is installed", tag)
	st.Phase = phaseInstalled
	if err := st.save(a.dir); err != nil {
		return false, err
	}
	os.Remove(iso) // a new install downloads the release current then
	return true, nil
}

// fetchImage downloads the latest patina image, or keeps the one downloaded already
// when it still matches the release's checksum.
func (a *app) fetchImage() (iso, tag string, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tag, err = latestRelease(ctx, a.http, patinaRepo)
	if err != nil {
		return "", "", fmt.Errorf("cannot reach patina's releases on GitHub: %w", err)
	}
	want, err := checksum(ctx, a.http, releaseAsset(patinaRepo, tag, sumsName), isoName)
	if err != nil {
		return "", "", err
	}
	iso = filepath.Join(a.dir, isoName)
	if fileExists(iso) {
		if got, _ := fileSHA256(iso); got == want {
			return iso, tag, nil
		}
		os.Remove(iso)
	}
	dlg, err := zenity.Progress(a.opts(zenity.MaxValue(1000), zenity.TimeRemaining(), zenity.Width(420))...)
	if err != nil {
		return "", "", err
	}
	defer dlg.Close()
	go func() {
		select {
		case <-dlg.Done():
			cancel()
		case <-ctx.Done():
		}
	}()
	dlg.Text("Downloading patina " + tag + "…")
	var last time.Time
	err = download(ctx, a.http, releaseAsset(patinaRepo, tag, isoName), iso, want, func(done, total int64) {
		if time.Since(last) < 250*time.Millisecond || total == 0 {
			return
		}
		last = time.Now()
		dlg.Value(int(done * 1000 / total))
		dlg.Text(fmt.Sprintf("Downloading patina %s… %d of %d MB", tag, done>>20, total>>20))
	})
	if err != nil {
		return "", "", err
	}
	dlg.Complete()
	a.log.Printf("downloaded %s %s, checksum %s", isoName, tag, want)
	return iso, tag, nil
}

// createDisk makes the empty virtual disk and the UEFI variables beside it.
func (a *app) createDisk() error {
	disk := filepath.Join(a.dir, diskFile)
	os.Remove(disk)
	out, err := hidden(exec.Command(filepath.Join(a.qemuDir, qemuImg), "create", "-q", "-f", "qcow2", disk, diskSize)).
		CombinedOutput()
	if err != nil {
		return fmt.Errorf("qemu-img could not create the disk: %v %s", err, out)
	}
	return copyFile(filepath.Join(a.qemuDir, "share", firmwareVarsBlank), filepath.Join(a.dir, varsFile))
}

// boot runs the machine until it stops and returns why it stopped. When the hypervisor
// refuses QEMU it tries again without the in-kernel interrupt controller, then offers
// emulation.
func (a *app) boot(iso string) (string, error) {
	for {
		reason, early, err := a.start(iso)
		if err != nil || !early {
			return reason, err
		}
		tail := lastLines(readFile(filepath.Join(a.dir, qemuLog)), 8)
		a.log.Printf("QEMU stopped at once with %s:\n%s", a.accel, tail)
		switch {
		case a.accel == accelWHPX && strings.Contains(strings.ToLower(tail), "whpx"):
			a.accel = accelWHPXNoIRQ
		case a.accel != accelEmulation && strings.Contains(strings.ToLower(tail), "whpx"):
			if zenity.Question("The Windows Hypervisor Platform refused to run patina:\n\n"+tail+
				"\n\nRun it without the hypervisor? It is much slower.",
				a.opts(zenity.OKLabel("Run slowly"), zenity.CancelLabel("Quit"), zenity.WarningIcon)...) != nil {
				return "", zenity.ErrCanceled
			}
			a.accel = accelEmulation
		default:
			return "", fmt.Errorf("QEMU stopped as it started:\n\n%s", tail)
		}
	}
}

// start runs QEMU once. early is true when it stopped within seconds without a
// shutdown, which is how a refused accelerator or a bad option shows.
func (a *app) start(iso string) (reason string, early bool, err error) {
	memoryMB, cpus := sizing(hostMemoryMB(), runtime.NumCPU())
	m := Machine{QEMUDir: a.qemuDir, DataDir: a.dir, MemoryMB: memoryMB, CPUs: cpus,
		Accel: a.accel, ISO: iso, QMP: freeAddr()}
	logf, err := os.Create(filepath.Join(a.dir, qemuLog))
	if err != nil {
		return "", false, err
	}
	defer logf.Close()
	cmd := exec.Command(filepath.Join(a.qemuDir, qemuSystem), m.Args()...)
	cmd.Dir = a.qemuDir
	cmd.Stdout, cmd.Stderr = logf, logf
	a.log.Printf("starting QEMU: %s", strings.Join(cmd.Args, " "))
	started := time.Now()
	if err := cmd.Start(); err != nil {
		return "", false, fmt.Errorf("could not start QEMU: %w", err)
	}
	exited := make(chan struct{})
	go func() {
		cmd.Wait()
		close(exited)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	go func() {
		select {
		case <-exited:
			cancel()
		case <-ctx.Done():
		}
	}()
	q, qerr := dialQMP(ctx, m.QMP)
	cancel()
	if qerr != nil {
		select {
		case <-exited:
			return "", true, nil // it stopped before it answered: the log says why
		default:
			cmd.Process.Kill()
			<-exited
			return "", false, fmt.Errorf("QEMU did not answer on its machine protocol: %w", qerr)
		}
	}
	a.running(q)
	a.status.SetTitle(fmt.Sprintf("%s: patina is running (%d GB, %d processors)", appName, memoryMB>>10, cpus))
	if iso == "" {
		zenity.Notify("patina is starting. To close it, shut it down from its own menu or from this icon.",
			a.opts()...)
	}
	for e := range q.Events() {
		if e.Event == "SHUTDOWN" {
			reason = shutdownReason(e)
			a.log.Printf("shutdown: %s", reason)
		}
	}
	q.Close()
	<-exited
	a.running(nil)
	return reason, reason == "" && time.Since(started) < 20*time.Second, nil
}

// freeAddr is a local TCP address no one listens on, for QEMU's QMP socket.
func freeAddr() string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "127.0.0.1:4444"
	}
	defer l.Close()
	return l.Addr().String()
}

func readFile(p string) string {
	b, _ := os.ReadFile(p)
	return string(b)
}

// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arc supports interacting with the ARC framework, which is used to run Android applications on Chrome OS.
package arc

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/caller"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/adb"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/chromeproc"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	// BootTimeout is the maximum amount of time that ARC is expected to take to boot.
	// Tests that call New should declare a timeout that's at least this long.
	BootTimeout = 120 * time.Second

	// Time Android init process takes to start. It should be smaller than BootTimeout.
	androidInitTimeout = 60 * time.Second

	intentHelperTimeout = 20 * time.Second

	// Time waiting for packages to install, for example enterprise auto install.
	waitPackagesTimeout = 5 * time.Minute

	logcatName = "logcat.txt"

	//ARCPath is the path where the container images are installed in the rootfs.
	ARCPath = "/opt/google/containers/android"
	//ARCVMPath is the pather where the VM images are installed in the rootfs.
	ARCVMPath = "/opt/google/vms/android"
)

// DisableSyncFlags is the default flags for disabling ARC content sync and background activities when using GAIA accounts.
// --arc-disable-app-sync  - prevents syncing installed apps from the previous sessions.
// --arc-disable-play-auto-install - disables PAI flow that downloads and installs apps in the background.
// --arc-play-store-auto-update=off - prevents Play Store and GMS Core from third party app update and prevents self-updates and downloadable content.
// --arc-disable-locale-sync - don’t propagate locale sync for the account that might cause reconfiguration updates.
// --arc-disable-media-store-maintenance - disables GMS scheduling of media store periodic indexing and corpora maintenance tasks.
func DisableSyncFlags() []string {
	return []string{"--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off", "--arc-disable-media-store-maintenance"}
}

// InstallType is the type of ARC (Container or VM) available on the device.
type InstallType int

const (
	// Container is for the ARC container install.
	Container InstallType = iota
	// VM is for the ARCVM install.
	VM
)

// locked is a flag that makes New and Close fail unconditionally.
var locked = false

// prePackages lists packages containing preconditions that are allowed to call
// Lock and Unlock.
var prePackages = []string{
	"chromiumos/tast/local/arc",
	"chromiumos/tast/local/multivm",
}

// Lock sets a flag that makes New and Close fail unconditionally.
// Preconditions and fixtures should call this function on setup to prevent
// tests from invalidating an ARC object by a mistake.
func Lock() {
	caller.Check(2, prePackages)
	locked = true
}

// Unlock resets the flag set by lock.
func Unlock() {
	caller.Check(2, prePackages)
	locked = false
}

// TODO(b/134144418): Consolidate ARC and ARCVM diverged code once ADB issues are resolved.

// Supported returns true if ARC is supported on the board.
//
// This function must not be used to skip tests entirely; declare the "android_p"
// software dependency instead. A valid use case would be to change the test
// expectation by whether ARC is supported or not (e.g. existence of mount
// points).
func Supported() bool {
	_, ok := Type()
	return ok
}

// Type detects the type (container or VM) of the ARC installation. As for
// Supported(), it should not be used to skip tests entirely, both fall under
// the "android_p" software dependency. But it could be used to change the
// behaviour of a test (e.g. check that ARCVM is running or not).
func Type() (t InstallType, ok bool) {
	if _, err := os.Stat(filepath.Join(ARCPath, "system.raw.img")); err == nil {
		return Container, true
	}
	if _, err := os.Stat(filepath.Join(ARCVMPath, "system.raw.img")); err == nil {
		return VM, true
	}
	return 0, false
}

// ARC holds resources related to an active ARC session. Call Close to release
// those resources.
type ARC struct {
	device       *adb.Device   // ADB device to communicate with ARC
	logcatCmd    *testexec.Cmd // process saving Android logs
	logcatWriter dynamicWriter // writes output from logcatCmd to logcatFile
	logcatFile   *os.File      // file currently being written to
}

// Close releases testing-related resources associated with ARC.
// ARC itself is not stopped.
func (a *ARC) Close(ctx context.Context) error {
	if locked {
		panic("Do not call Close while precondition is being used")
	}
	var err error
	if a.logcatCmd != nil {
		a.logcatCmd.Kill()
		a.logcatCmd.Wait()
	}
	if a.logcatFile != nil {
		a.logcatWriter.setDest(nil)
		err = a.logcatFile.Close()
	}
	return err
}

// New waits for Android to finish booting.
//
// ARC must be enabled in advance by passing chrome.ARCEnabled or chrome.ARCSupported with
// real user gaia login to chrome.New.
//
// After this function returns successfully, you can assume BOOT_COMPLETED
// intent has been broadcast from Android system, and ADB connection is ready.
// Note that this does not necessarily mean all ARC mojo services are up; call
// WaitIntentHelper() to wait for ArcIntentHelper to be ready, for example.
//
// The returned ARC instance must be closed when the test is finished.
func New(ctx context.Context, outDir string) (*ARC, error) {
	ctx, st := timing.Start(ctx, "arc_new")
	defer st.End()

	if locked {
		panic("Cannot create ARC instance while precondition is being used")
	}

	ctx, cancel := context.WithTimeout(ctx, BootTimeout)
	defer cancel()

	if err := checkSoftwareDeps(ctx); err != nil {
		return nil, err
	}

	if err := ensureARCEnabled(); err != nil {
		return nil, err
	}

	arc := &ARC{}
	toClose := arc
	defer func() {
		if toClose != nil {
			toClose.Close(ctx)
		}
	}()

	testing.ContextLog(ctx, "Waiting for Android boot")

	// Prepare logcat file, it may be empty if early boot fails.
	logcatPath := filepath.Join(outDir, logcatName)
	if err := arc.setLogcatFile(logcatPath); err != nil {
		return nil, errors.Wrap(err, "failed to create logcat output file")
	}

	if err := WaitAndroidInit(ctx); err != nil {
		// Try starting logcat just in case logcat is possible. Android might still be up.
		logcatCmd := BootstrapCommand(ctx, "/system/bin/logcat", "-d")
		logcatCmd.Stdout = &arc.logcatWriter
		testing.ContextLog(ctx, "Forcing collection of logcat at early boot")
		if err := logcatCmd.Run(); err != nil {
			testing.ContextLog(ctx, "Tried starting logcat anyway but failed: ", err)
		}
		return nil, diagnose(logcatPath, errors.Wrap(err, "Android failed to boot in very early stage"))
	}

	// At this point we can start logcat.
	logcatCmd, err := startLogcat(ctx, &arc.logcatWriter)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start logcat")
	}
	arc.logcatCmd = logcatCmd

	// This property is set by the Android system server just before LOCKED_BOOT_COMPLETED is broadcast.
	const androidBootProp = "sys.boot_completed"
	if err := waitProp(ctx, androidBootProp, "1", reportTiming); err != nil {
		return nil, diagnose(logcatPath, errors.Wrapf(err, "%s not set", androidBootProp))
	}

	var ch chan error
	// Android is up. Set up ADB connection in parallel to Android boot since
	// ADB local server takes a few seconds to start up.
	testing.ContextLog(ctx, "Setting up ADB connection")
	ch = make(chan error, 1)
	go func() {
		ch <- adb.LaunchServer(ctx)
	}()

	// This property is set by ArcAppLauncher when it receives BOOT_COMPLETED.
	const arcBootProp = "ro.arc.boot_completed"
	if err := waitProp(ctx, arcBootProp, "1", reportTiming); err != nil {
		return nil, diagnose(logcatPath, errors.Wrapf(err, "%s not set", arcBootProp))
	}

	// Android has booted.
	if err := <-ch; err != nil {
		return nil, diagnose(logcatPath, errors.Wrap(err, "failed setting up ADB auth"))
	}

	// Connect to ADB.
	device, err := connectADB(ctx)
	if err != nil {
		return nil, diagnose(logcatPath, errors.Wrap(err, "failed connecting to ADB"))
	}
	arc.device = device

	toClose = nil
	return arc, nil
}

// WaitIntentHelper waits for ArcIntentHelper to get ready.
func (a *ARC) WaitIntentHelper(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, intentHelperTimeout)
	defer cancel()

	testing.ContextLog(ctx, "Waiting for ArcIntentHelper")
	const prop = "ro.arc.intent_helper.ready"
	if err := waitProp(ctx, prop, "1", reportTiming); err != nil {
		return errors.Wrapf(err, "property %s not set", prop)
	}
	return nil
}

// androidDeps contains Android-related software features (see testing.Test.SoftwareDeps).
// At least one of them must be declared to call New.
var androidDeps = []string{
	"android_vm",
	"android_vm_r",
	"android_p",
	"arc",
}

// checkSoftwareDeps ensures the current test declares Android software dependencies.
func checkSoftwareDeps(ctx context.Context) error {
	deps, ok := testing.ContextSoftwareDeps(ctx)
	if !ok {
		// Test info can be unavailable in unit tests.
		return nil
	}

	for _, dep := range deps {
		for _, adep := range androidDeps {
			if dep == adep {
				return nil
			}
		}
	}
	return errors.Errorf("test must declare at least one of Android software dependencies %v", androidDeps)
}

// setLogcatFile creates a new logcat output file at p and opens it as a.logcatFile.
// a.logcatWriter is updated to write to the new file, and explanatory messages are
// written to both the new file and old file (if there was a previous file).
func (a *ARC) setLogcatFile(p string) error {
	oldFile := a.logcatFile

	var createErr error
	a.logcatFile, createErr = os.Create(p)
	if createErr == nil && oldFile != nil {
		// Make the new file start with a line pointing at the old file.
		if rel, err := filepath.Rel(filepath.Dir(a.logcatFile.Name()), oldFile.Name()); err == nil {
			fmt.Fprintf(a.logcatFile, "[output continued from %v]\n", rel)
		}
	}
	// If the create failed, we'll just drop the new logs.
	a.logcatWriter.setDest(a.logcatFile)

	if oldFile != nil {
		if a.logcatFile != nil {
			// Make the old file end with a line pointing at the new file.
			if rel, err := filepath.Rel(filepath.Dir(oldFile.Name()), a.logcatFile.Name()); err == nil {
				fmt.Fprintf(oldFile, "[output continued in %v]\n", rel)
			}
		}
		oldFile.Close()
	}

	return createErr
}

// VMEnabled returns true if Chrome OS is running ARCVM.
func VMEnabled() (bool, error) {
	installType, ok := Type()
	if !ok {
		return false, errors.New("failed to get installation type")
	}
	return installType == VM, nil
}

// ensureARCEnabled makes sure ARC is enabled by a command line flag to Chrome.
func ensureARCEnabled() error {
	args, err := getChromeArgs()
	if err != nil {
		return errors.Wrap(err, "failed getting Chrome args")
	}

	for _, a := range args {
		if a == "--arc-start-mode=always-start-with-no-play-store" || a == "--arc-availability=officially-supported" {
			return nil
		}
	}
	return errors.New("ARC is not enabled; pass chrome.ARCEnabled or chrome.ARCSupported to chrome.New")
}

// getChromeArgs returns command line arguments of the Chrome browser process.
func getChromeArgs() ([]string, error) {
	pid, err := chromeproc.GetRootPID()
	if err != nil {
		return nil, err
	}
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return nil, err
	}
	return proc.CmdlineSlice()
}

// diagnoseInitfailure extracts significant logs for init failure,
// such as exit message from crosvm.
func diagnoseInitfailure(reader *syslog.Reader, observedErr error) error {
	lastMessage := ""
	for {
		entry, err := reader.Read()
		if err != nil {
			// End of syslog is reached (io.EOF) or some other error
			// happened.  Either way, return with last significant
			// message if available.
			if lastMessage != "" {
				return errors.Wrapf(observedErr, "%v", lastMessage)
			}
			return observedErr
		}
		if strings.HasPrefix(entry.Program, "ARCVM") {
			// TODO(b/167944318): try a better message
			lastMessage = entry.Content
		}
	}
}

// WaitAndroidInit waits for Android init process to start.
//
// It is very rare you want to call this function from your test; to wait for
// the Android system to start and become ready, call New instead. A valid use
// case is when you want to interact with Android mini container.
//
// It is fine to call BootstrapCommand after this function successfully returns.
func WaitAndroidInit(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, androidInitTimeout)
	defer cancel()

	// Start a syslog reader so we can give more useful debug
	// information waiting for boot.
	reader, err := syslog.NewReader(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open syslog reader")
	}
	defer reader.Close()

	// Wait for init or crosvm process to start before checking deeper.
	testing.ContextLog(ctx, "Waiting for initial ARC process")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := InitPID()
		return err
	}, &testing.PollOptions{Interval: time.Second}); err != nil {
		return diagnoseInitfailure(reader, errors.Wrap(err, "init/crosvm process did not start up"))
	}

	// Wait for an arbitrary property set by Android init very
	// early in "on boot". Wait for it to ensure Android init
	// process started.
	const prop = "net.tcp.default_init_rwnd"
	if err := waitProp(ctx, prop, "60", reportTiming); err != nil {
		// Check if init/crosvm is still alive at this point.
		if _, err := InitPID(); err != nil {
			return errors.Wrap(err, "init/crosvm process exited unexpectedly")
		}
		return errors.Wrapf(err, "%s property is not set which shows that Android init did not come up", prop)
	}
	return nil
}

// startLogcat starts a logcat process with its stdout redirected to w.
func startLogcat(ctx context.Context, w io.Writer) (*testexec.Cmd, error) {
	// Wait for logd to start by polling logcat.
	cmd := BootstrapCommand(ctx, "/system/bin/sh", "-c", "while ! /system/bin/logcat -ds; do sleep 0.1; done")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrap(err, "logcat failed")
	}

	// The logcat process may need to span multiple tests if we're being used by a precondition,
	// so use context.Background instead of ctx to make sure it isn't killed prematurely.
	cmd = BootstrapCommand(context.Background(), "/system/bin/logcat") // NOLINT: process may need to persist across multiple tests
	cmd.Stdout = w
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

// timingMode describes whether timing information should be reported.
type timingMode int

const (
	reportTiming   timingMode = iota // create a timing stage
	noReportTiming                   // don't create a timing stage
)

// waitProp waits for Android prop name is set to value.
func waitProp(ctx context.Context, name, value string, tm timingMode) error {
	if tm == reportTiming {
		var st *timing.Stage
		ctx, st = timing.Start(ctx, fmt.Sprintf("wait_prop_%s=%s", name, value))
		defer st.End()
	}

	const loop = `while [ "$(/system/bin/getprop "$1")" != "$2" ]; do sleep 0.1; done`
	return testing.Poll(ctx, func(ctx context.Context) error {
		return BootstrapCommand(ctx, "/system/bin/sh", "-c", loop, "-", name, value).Run()
	}, &testing.PollOptions{Interval: time.Second})
}

// APKPath returns the absolute path to a helper APK.
func APKPath(value string) string {
	return adb.APKPath(value)
}

// makeList returns a list of keys from map.
func makeList(packages map[string]bool) []string {
	var packagesList []string
	for pkg := range packages {
		packagesList = append(packagesList, pkg)
	}
	sort.Strings(packagesList)
	return packagesList
}

// WaitForPackages waits for Android packages being installed.
func (a *ARC) WaitForPackages(ctx context.Context, packages []string) error {
	ctx, st := timing.Start(ctx, "wait_packages")
	defer st.End()

	ctx, cancel := context.WithTimeout(ctx, waitPackagesTimeout)
	defer cancel()

	notInstalledPackages := make(map[string]bool)
	for _, p := range packages {
		notInstalledPackages[p] = true
	}

	return testing.Poll(ctx, func(ctx context.Context) error {
		pkgs, err := a.InstalledPackages(ctx)
		if err != nil {
			return testing.PollBreak(err)
		}

		for p := range pkgs {
			if notInstalledPackages[p] {
				delete(notInstalledPackages, p)
			}
		}
		if len(notInstalledPackages) != 0 {
			return errors.Errorf("%d package(s) are not installed yet: %s",
				len(notInstalledPackages),
				strings.Join(makeList(notInstalledPackages), ", "))
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second})
}

// State holds the ARC state returned from autotestPrivate.getArcState() call.
//
// Refer to https://chromium.googlesource.com/chromium/src/+/master/chrome/common/extensions/api/autotest_private.idl
// for the mapping of the fields to JavaScript.
type State struct {
	// Provisioned indicates whether the ARC is provisioned.
	Provisioned bool `json:"provisioned"`
	// TOSNeeded indicates whether ARC Terms of Service needs to be shown.
	TOSNeeded bool `json:"tosNeeded"`
	// PreStartTime is ARC pre-start time (mini-ARC) or 0 if not pre-started.
	PreStartTime float64 `json:"preStartTime"`
	// StartTime is the ARC start time or 0 if not started.
	StartTime float64 `json:"startTime"`
}

// GetState gets the arc state. It is a wrapper for chrome.autotestPrivate.getArcState.
func GetState(ctx context.Context, tconn *chrome.TestConn) (State, error) {
	var state State
	if err := tconn.Call(ctx, &state, `tast.promisify(chrome.autotestPrivate.getArcState)`); err != nil {
		return state, errors.Wrap(err, "failed to run autotestPrivate.getArcState")
	}
	return state, nil
}

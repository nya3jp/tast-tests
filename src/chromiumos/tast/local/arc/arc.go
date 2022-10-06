// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arc supports interacting with the ARC framework, which is used to run Android applications on ChromeOS.
package arc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"chromiumos/tast/caller"
	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	localadb "chromiumos/tast/local/android/adb"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash/ashproc"
	"chromiumos/tast/local/procutil"
	"chromiumos/tast/local/syslog"
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

	logcatName = "logcat.txt"

	pstoreCommandPath                 = "/usr/bin/vm_pstore_dump"
	pstoreCommandExitCodeFileNotFound = 2
	arcvmConsoleName                  = "messages-arcvm"
	arcLogPath                        = "/var/log/arc.log"

	//ARCPath is the path where the container images are installed in the rootfs.
	ARCPath = "/opt/google/containers/android"
	//ARCVMPath is the pather where the VM images are installed in the rootfs.
	ARCVMPath = "/opt/google/vms/android"

	virtioBlkDataPropName = "ro.boot.arcvm_virtio_blk_data"
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
	"chromiumos/tast/local/bundles/crosint/arc",
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
	outDir       string        // directory for log files. This becomes empty after ARC.SaveLogFiles().
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
	var errs []error
	if err := a.SaveLogFiles(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := a.cleanUpLogcatFile(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) != 0 {
		return errs[0]
	}
	return nil
}

// WaitForProvisioning waits for ARC to be provisioned.
func (a *ARC) WaitForProvisioning(ctx context.Context, timeout time.Duration) error {
	testing.ContextLog(ctx, "Waiting for provisioning")
	return testing.Poll(ctx, func(ctx context.Context) error {
		if result, err := a.IsProvisioned(ctx); err != nil {
			return testing.PollBreak(err)
		} else if !result {
			return errors.New("ARC not yet provisioned")
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: timeout})
}

// IsProvisioned returns true if the provisioning is complete.
func (a *ARC) IsProvisioned(ctx context.Context) (bool, error) {
	res, err := a.Command(ctx, "settings", "get", "global", "device_provisioned").Output(testexec.DumpLogOnError)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(res)) == "1", nil
}

// ReadXMLFile reads XML file and converts from binary to plain-text if necessary.
func (a *ARC) ReadXMLFile(ctx context.Context, filepath string) ([]byte, error) {
	out, err := ioutil.ReadFile(filepath)
	if err != nil || len(out) == 0 || bytes.HasPrefix(out, []byte("<?xml ")) {
		return out, err
	}
	if isVMEnabled, err := VMEnabled(); err != nil {
		return nil, err
	} else if isVMEnabled {
		out, err = a.Abx2Xml(ctx, out)
	}
	return out, err
}

// Abx2Xml converts binary XML to plain-text XML.
func (a *ARC) Abx2Xml(ctx context.Context, data []byte) ([]byte, error) {
	cmd := a.Command(ctx, "abx2xml", "-", "-")
	cmd.Stdin = bytes.NewBuffer(data)
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "abx2xml failed")
	}
	return out, nil
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
	// Start a syslog reader so we can give more useful debug information
	// waiting for boot. This is too late in the boot to
	// catch crosvm startup crashes.
	reader, err := syslog.NewReader(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open syslog reader")
	}
	defer reader.Close()

	return NewWithSyslogReader(ctx, outDir, reader)
}

// NewWithTimeout waits for Android to finish booting until timeout expires.
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
func NewWithTimeout(ctx context.Context, outDir string, timeout time.Duration) (*ARC, error) {
	// Start a syslog reader so we can give more useful debug information
	// waiting for boot. This is too late in the boot to
	// catch crosvm startup crashes.
	reader, err := syslog.NewReader(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open syslog reader")
	}
	defer reader.Close()

	return newWithSyslogReaderAndTimeout(ctx, outDir, reader, timeout)
}

// newWithSyslogReaderAndTimeout waits for Android to finish booting until timeout expires.
//
// Give a syslog.Reader instantiated before chrome.New to allow diagnosing init failure.
func newWithSyslogReaderAndTimeout(ctx context.Context, outDir string, reader *syslog.Reader, timeout time.Duration) (a *ARC, retErr error) {
	defer func() {
		if retErr == nil {
			return
		}

		// We hit unexpected cases that we found two or more chrome root processes.
		// To investigate the case, we dump the all processes here.
		// TODO(b/231683759): Remove the dump when the problem is solved.
		var err *procutil.FoundTooManyProcessesError
		if !errors.As(retErr, &err) {
			// The error is different from what we're interested in.
			return
		}

		outDir, ok := testing.ContextOutDir(ctx)
		if !ok {
			testing.ContextLog(ctx, "outdir not found")
			return
		}

		var lines []string
		for _, p := range err.All {
			exe, err := p.Exe()
			if err != nil {
				testing.ContextLogf(ctx, "Exe not found for %v: %v", p.Pid, err)
			}
			cmdline, err := p.Cmdline()
			if err != nil {
				testing.ContextLogf(ctx, "Cmdline not found for %v: %v", p.Pid, err)
			}
			ppid, err := p.Ppid()
			if err != nil {
				testing.ContextLogf(ctx, "Ppid not found for %v: %v", p.Pid, err)
			}
			line := fmt.Sprintf("%v, %q, %q, %v", p.Pid, exe, cmdline, ppid)
			// Note: we hit an issue that the file written below got disappeared
			// for some reasons. The issue is investigated, but this part of the code is also
			// for the investigation of another test flakiness. To investigate the issues in parallel,
			// we dump the info to log, which is not recommended in general, though.
			testing.ContextLog(ctx, line)
			lines = append(lines, line+"\n")
		}
		path := filepath.Join(outDir, "chromeroot-ps.txt")
		if err := os.WriteFile(path, []byte(strings.Join(lines, "")), 0644); err != nil {
			testing.ContextLog(ctx, "Failed to dump chromeroot-ps.txt: ", err)
		}
	}()

	ctx, st := timing.Start(ctx, "arc_new")
	defer st.End()

	if locked {
		panic("Cannot create ARC instance while precondition is being used")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := checkSoftwareDeps(ctx); err != nil {
		return nil, err
	}

	if err := ensureARCEnabled(ctx); err != nil {
		return nil, err
	}

	arc := &ARC{
		outDir: outDir,
	}
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

	if err := WaitAndroidInit(ctx, reader); err != nil {
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
		ch <- localadb.LaunchServer(ctx)
	}()

	// This property is set by ArcAppLauncher when it receives BOOT_COMPLETED.
	const arcBootProp = "ro.vendor.arc.boot_completed"
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

	// Disable the Play Store package entirely unless it's booted with chrome.ARCSupported().
	if enabled, err := isPlayStoreEnabled(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to check whether Play Store is enabled")
	} else if !enabled {
		pkgs, err := arc.InstalledPackages(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get installed packages")
		}

		if _, found := pkgs["com.android.vending"]; found {
			testing.ContextLog(ctx, "Disabling Play Store")
			if err := arc.Command(ctx, "pm", "disable-user", "--user", "0", "com.android.vending").Run(); err != nil {
				return nil, errors.Wrap(err, "failed to disable Play Store")
			}
		}
	}

	toClose = nil
	return arc, nil
}

// NewWithSyslogReader waits for Android to finish booting.
//
// Give a syslog.Reader instantiated before chrome.New to allow diagnosing init failure.
func NewWithSyslogReader(ctx context.Context, outDir string, reader *syslog.Reader) (*ARC, error) {
	return newWithSyslogReaderAndTimeout(ctx, outDir, reader, BootTimeout)
}

// WaitIntentHelper waits for ArcIntentHelper to get ready.
func (a *ARC) WaitIntentHelper(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, intentHelperTimeout)
	defer cancel()

	testing.ContextLog(ctx, "Waiting for ArcIntentHelper")
	const prop = "ro.vendor.arc.intent_helper.ready"
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

// ResetOutDir updates the outDir field of ARC object.
func (a *ARC) ResetOutDir(ctx context.Context, outDir string) error {
	a.outDir = outDir
	if err := a.setLogcatFile(filepath.Join(a.outDir, logcatName)); err != nil {
		return err
	}
	return nil
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

func (a *ARC) cleanUpLogcatFile() error {
	if a.logcatCmd != nil {
		a.logcatCmd.Kill()
		a.logcatCmd.Wait()
	}
	if a.logcatFile != nil {
		a.logcatWriter.setDest(nil)
		if err := a.logcatFile.Close(); err != nil {
			return err
		}
	}
	return nil
}

// VMEnabled returns true if ChromeOS is running ARCVM.
func VMEnabled() (bool, error) {
	installType, ok := Type()
	if !ok {
		return false, errors.New("failed to get installation type")
	}
	return installType == VM, nil
}

// ensureARCEnabled makes sure ARC is enabled by a command line flag to Chrome.
func ensureARCEnabled(ctx context.Context) error {
	args, err := chromeArgsWithContext(ctx)
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

// isPlayStoreEnabled returns true when Play Store is enabled i.e. chrome.ARCSupported is passed.
func isPlayStoreEnabled(ctx context.Context) (bool, error) {
	args, err := chromeArgsWithContext(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed getting Chrome args")
	}

	for _, a := range args {
		if a == "--arc-start-mode=always-start-with-no-play-store" {
			return false, nil
		}
	}
	return true, nil
}

// IsVirtioBlkDataEnabled returns whether ARCVM virtio-blk /data is enabled on the device.
func (a *ARC) IsVirtioBlkDataEnabled(ctx context.Context) (bool, error) {
	out, err := a.GetProp(ctx, virtioBlkDataPropName)
	if err != nil {
		return false, errors.Wrap(err, "failed to get prop for arcvm_virtio_blk_data")
	}
	return out == "1", nil
}

// chromeArgs returns command line arguments of the Chrome browser process.
func chromeArgs() ([]string, error) {
	proc, err := ashproc.Root()
	if err != nil {
		return nil, err
	}
	return proc.CmdlineSlice()
}

func chromeArgsWithContext(ctx context.Context) ([]string, error) {
	proc, err := ashproc.RootWithContext(ctx)
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
				return errors.Wrap(observedErr, lastMessage)
			}
			return observedErr
		}
		if entry.Program == "crash_reporter" && strings.Contains(entry.Content, "Received crash notification for crosvm") {
			return errors.Wrap(observedErr, entry.Content)
		}
		if strings.HasPrefix(entry.Program, "ARCVM") && strings.Contains(entry.Content, "crosvm has exited with error: ") {
			return errors.Wrap(observedErr, entry.Content)
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
func WaitAndroidInit(ctx context.Context, reader *syslog.Reader) error {
	ctx, cancel := context.WithTimeout(ctx, androidInitTimeout)
	defer cancel()

	// Wait for init or crosvm process to start before checking deeper.
	testing.ContextLog(ctx, "Waiting for initial ARC process")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := InitPID()
		return err
	}, &testing.PollOptions{Interval: time.Second}); err != nil {
		return diagnoseInitfailure(reader, errors.Wrap(err, "init/crosvm process did not start up"))
	}

	// Wait for property set by Android init in early stages. For P, wait for net.tcp.default_init_rwnd.
	// For R and later, wait for ro.vendor.arc.on_boot which is set while bertha device is on boot.
	// TODO(b/185198563): Replace net.tcp.default_init_rwnd with ro.vendor.arc.on_boot completely.
	isVMEnabled, err := VMEnabled()
	if err != nil {
		return err
	}

	var prop = "ro.vendor.arc.on_boot"
	var value = "1"
	if !isVMEnabled {
		prop = "net.tcp.default_init_rwnd"
		value = "60"
	}

	if err := waitProp(ctx, prop, value, reportTiming); err != nil {
		// Check if init/crosvm is still alive at this point.
		if _, err := InitPID(); err != nil {
			return diagnoseInitfailure(reader, errors.Wrap(err, "init/crosvm process exited unexpectedly"))
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

	// TODO(b/237255015): HACK Remove legacy_name alternative when all ARC
	// branches are using the correctly-namespaced properties.
	legacyNameHack := strings.Replace(name, "ro.vendor.arc.", "ro.arc.", 1)
	const loop = `
while [ "$(/system/bin/getprop "$1")" != "$2" ] &&
      [ "$(/system/bin/getprop "$3")" != "$2" ]
do
	sleep 0.1
done`

	return testing.Poll(ctx, func(ctx context.Context) error {
		return BootstrapCommand(ctx, "/system/bin/sh", "-c", loop, "-", name, value, legacyNameHack).Run()
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

// frozenPackages lists all packages that are frozen.
func (a *ARC) frozenPackages(ctx context.Context) (map[string]struct{}, error) {
	out, err := a.Command(ctx, "dumpsys", "package", "frozen").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, err
	}

	packages := make(map[string]struct{})
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	// Skip the first line with title 'Frozen packages:'.
	for _, name := range lines[1:] {
		name = strings.TrimSpace(name)
		packages[name] = struct{}{}
	}
	return packages, nil
}

// WaitForPackages waits for Android packages being installed.
func (a *ARC) WaitForPackages(ctx context.Context, packages []string) error {
	ctx, st := timing.Start(ctx, "wait_packages")
	defer st.End()

	notInstalledPackages := make(map[string]bool)
	for _, p := range packages {
		notInstalledPackages[p] = true
	}

	testing.ContextLog(ctx, "Waiting for packages")

	return testing.Poll(ctx, func(ctx context.Context) error {
		pkgs, err := a.InstalledPackages(ctx)
		if err != nil {
			// Package service may not be running yet. Wait until it's available.
			if strings.Contains(err.Error(), "package service not running") {
				return err
			}
			return testing.PollBreak(err)
		}

		frozenPkgs, err := a.frozenPackages(ctx)
		if err != nil {
			return testing.PollBreak(err)
		}

		for p := range pkgs {
			// A package is not fully installed until is unfrozen.
			if _, frozen := frozenPkgs[p]; !frozen && notInstalledPackages[p] {
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
// Refer to https://chromium.googlesource.com/chromium/src/+/main/chrome/common/extensions/api/autotest_private.idl
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

// SaveLogFiles writes log files to the a.outDir directory and clears the a.outDir.
func (a *ARC) SaveLogFiles(ctx context.Context) error {
	if a.outDir == "" {
		return nil
	}
	if _, err := os.Stat(a.outDir); os.IsNotExist(err) {
		// Preconditions may call this method after the outDir is removed.
		return nil
	}

	if err := saveARCVMConsole(ctx, filepath.Join(a.outDir, arcvmConsoleName)); err != nil {
		return errors.Wrap(err, "failed to save the messages-arcvm")
	}

	if err := fsutil.CopyFile(arcLogPath, filepath.Join(a.outDir, "arc.log")); err != nil {
		return errors.Wrap(err, "failed to save arc.log")
	}

	// Reset outDir to avoid saving the same files twice at ARC.Close().
	a.outDir = ""
	return nil
}

// saveARCVMConsole saves the console output of ARCVM Kernel to the given path using vm_pstore_dump command.
func saveARCVMConsole(ctx context.Context, path string) error {
	// Do nothing for containers. The console output is already captured for containers.
	isVMEnabled, err := VMEnabled()
	if err != nil {
		return err
	}
	if !isVMEnabled {
		return nil
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	cmd := testexec.CommandContext(ctx, pstoreCommandPath)
	cmd.Stdout = file
	var errbuf bytes.Buffer
	cmd.Stderr = &errbuf
	if err := cmd.Run(); err != nil {
		errmsg := errbuf.String()
		if cmd.ProcessState.ExitCode() == pstoreCommandExitCodeFileNotFound {
			// This failure sometimes happens when ARCVM failed to boot. So we don't make this error.
			testing.ContextLogf(ctx, "vm_pstore_dump command failed because the .pstore file doesn't exist: %#v", errmsg)
		} else {
			return errors.Wrapf(err, "vm_pstore_dump command failed with an unexpected reason: %#v", errmsg)
		}
	}
	return nil
}

const (
	arcvmDevConfFile       = "/usr/local/vms/etc/arcvm_dev.conf"
	arcvmDevConfFileBackup = "/usr/local/vms/etc/arcvm_dev.conf.tast-backup"
)

// WriteArcvmDevConf writes string to arcvm_dev.conf on ARCVM devices. Useful for modifying flags
// for crosvm start up. Backs up original content to arcvm_dev.conf.tast-backup which should
// later be restored with RestoreArcvmDevConf. Care should be taken to only write or append to the
// config once during a test run (either from the test itself or from the fixture) to avoid
// overwriting the backup config file.
func WriteArcvmDevConf(ctx context.Context, text string) error {
	isVMEnabled, err := VMEnabled()
	if err != nil {
		return err
	}
	if !isVMEnabled {
		return nil
	}

	// It's possible a previous test run didn't successfully clean up the backup. Restoring it here
	// should be safe as a test should only write or append to the config once.
	if _, err := os.Stat(arcvmDevConfFileBackup); err == nil {
		testing.ContextLogf(ctx,
			"%s already exists, restoring config from backup as the previous test might have failed to clean up",
			arcvmDevConfFileBackup)
		RestoreArcvmDevConf(ctx)
	}

	if err := os.Rename(arcvmDevConfFile, arcvmDevConfFileBackup); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			testing.ContextLog(ctx, "Original arcvm_dev.conf did not exist. Proceeding without backup")
		} else {
			return err
		}
	}
	return ioutil.WriteFile(arcvmDevConfFile, []byte(text), 0644)
}

// AppendToArcvmDevConf appends the string to the arcvm_dev.conf on ARCVM devices. Useful for
// modifying crosvm flags. Backs up the original content to arcvm_dev.conf.tast-backup which should
// later be restored with RestoreArcvmDevConf. Care should be taken to only write or append to the
// config once during a test run (either from the test itself or from the fixture) to avoid
// overwriting the backup config file.
func AppendToArcvmDevConf(ctx context.Context, text string) error {
	isVMEnabled, err := VMEnabled()
	if err != nil {
		return err
	}
	if !isVMEnabled {
		return nil
	}

	// It's possible a previous test run didn't successfully clean up the backup. Restoring it here
	// should be safe as a test should only write or append to the config once.
	if _, err := os.Stat(arcvmDevConfFileBackup); err == nil {
		testing.ContextLogf(ctx,
			"%s already exists, restoring config from backup as the previous test might have failed to clean up",
			arcvmDevConfFileBackup)
		RestoreArcvmDevConf(ctx)
	}

	// Copy arcvm_dev.conf to backup file.
	f, err := os.OpenFile(arcvmDevConfFile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	backup, err := os.OpenFile(arcvmDevConfFileBackup, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer backup.Close()
	if _, err = io.Copy(backup, f); err != nil {
		return err
	}
	if err = backup.Sync(); err != nil {
		return err
	}

	// Append config to arcvm_dev.conf.
	_, err = f.WriteString(text)
	return err
}

// RestoreArcvmDevConf restores the original arcvm_dev.conf from the backup copy set
// aside by WriteArcvmDevConf.
func RestoreArcvmDevConf(ctx context.Context) error {
	isVMEnabled, err := VMEnabled()
	if err != nil {
		return err
	}
	if !isVMEnabled {
		return nil
	}

	if err := os.Remove(arcvmDevConfFile); err != nil {
		return err
	}

	if err := os.Rename(arcvmDevConfFileBackup, arcvmDevConfFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			testing.ContextLog(ctx, "No backup, presumably because original arcvm_dev.conf did not exist. Proceeding without restoring backup")
		} else {
			return err
		}
	}
	return nil
}

// CheckNoDex2Oat verifies whether ARC is pre-optimized and no dex2oat was previously running in the background.
func CheckNoDex2Oat(outDir string) error {
	const (
		containerDexPrefix = `/system/bin/dex2oat .*--dex-file=(.+?) --`
		vmDexPrefix        = `DexInv: --- BEGIN \'(.+?)\' ---`
	)

	isVMEnabled, err := VMEnabled()
	if err != nil {
		return errors.Wrap(err, "failed to get whether ARCVM is enabled")
	}

	// check logcat for evidence of dex2oat running.
	logcatPath := filepath.Join(outDir, "logcat.txt")

	dump, err := ioutil.ReadFile(logcatPath)
	if err != nil {
		return errors.Wrap(err, "failed to read logcat")
	}

	dexPrefix := containerDexPrefix
	if isVMEnabled {
		dexPrefix = vmDexPrefix
	}
	m := regexp.MustCompile(dexPrefix).FindAllStringSubmatch(string(dump), -1)
	for _, match := range m {
		res := match[1]
		if !strings.HasPrefix(res, "/data/") {
			return errors.Errorf("failed due to system resource %q not pre-optimized", res)
		}
	}

	return nil
}

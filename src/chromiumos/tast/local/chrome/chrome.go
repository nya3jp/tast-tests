// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chrome implements a library used for communication with Chrome.
package chrome

import (
	"compress/gzip"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace"
	"github.com/golang/protobuf/proto"

	"chromiumos/tast/caller"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/internal/config"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/chrome/internal/extension"
	"chromiumos/tast/local/chrome/internal/login"
	"chromiumos/tast/local/chrome/jslog"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/minidump"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	// LoginTimeout is the maximum amount of time that Chrome is expected to take to perform login.
	// Tests that call New with the default fake login mode should declare a timeout that's at least this long.
	LoginTimeout = 80 * time.Second

	// gaiaLoginTimeout is the maximum amount of the time that Chrome is expected
	// to take to perform actual gaia login. As far as I checked a few samples of
	// actual test runs, most of successful login finishes within ~40secs. Use
	// 40*3=120 seconds for the safety.
	gaiaLoginTimeout = 120 * time.Second

	// uiRestartTimeout is the maximum amount of time that it takes to restart
	// the ui upstart job.
	// ui-post-stop can sometimes block for an extended period of time
	// waiting for "cryptohome --action=pkcs11_terminate" to finish: https://crbug.com/860519
	uiRestartTimeout = 60 * time.Second

	// TestExtensionID is an extension ID of the autotest extension. It
	// corresponds to testExtensionKey.
	TestExtensionID = extension.TestExtensionID

	// BlankURL is the URL corresponding to the about:blank page.
	BlankURL = "about:blank"

	// Virtual keyboard background page url.
	vkBackgroundPageURL = "chrome-extension://jkghodnilhceideoidjikpgommlajknk/background.html"
)

// locked is set to true while a precondition is active to prevent tests from calling New or Chrome.Close.
var locked = false

// prePackages lists packages containing preconditions that are allowed to call Lock and Unlock.
var prePackages = []string{
	"chromiumos/tast/local/arc",
	"chromiumos/tast/local/policyutil/pre",
	"chromiumos/tast/local/bundles/cros/camera/testutil",
	"chromiumos/tast/local/bundles/cros/ui/cuj",
	"chromiumos/tast/local/bundles/cros/inputs/pre",
	"chromiumos/tast/local/bundles/crosint/pita/pre",
	"chromiumos/tast/local/bundles/pita/pita/pre",
	"chromiumos/tast/local/chrome",
	"chromiumos/tast/local/crostini",
	"chromiumos/tast/local/drivefs",
	"chromiumos/tast/local/lacros/launcher",
	"chromiumos/tast/local/multivm",
	"chromiumos/tast/local/wpr",
}

// Lock prevents from New or Chrome.Close from being called until Unlock is called.
// It can only be called by preconditions and is idempotent.
func Lock() {
	caller.Check(2, prePackages)
	locked = true
}

// Unlock allows New and Chrome.Close to be called after an earlier call to Lock.
// It can only be called by preconditions and is idempotent.
func Unlock() {
	caller.Check(2, prePackages)
	locked = false
}

// Chrome interacts with the currently-running Chrome instance via the
// Chrome DevTools protocol (https://chromedevtools.github.io/devtools-protocol/).
type Chrome struct {
	// cfg contains configurations computed from options given to chrome.New.
	// Its fields must not be altered after its construction.
	cfg  config.Config
	exts *extension.Files
	agg  *jslog.Aggregator
	sess *driver.Session

	loginPending bool // true if login is pending until ContinueLogin is called
}

// User returns the username that was used to log in to Chrome.
func (c *Chrome) User() string { return c.cfg.User }

// DeprecatedExtDirs returns the directories holding the test extensions.
//
// DEPRECATED: This method does not handle sign-in profile extensions correctly.
func (c *Chrome) DeprecatedExtDirs() []string {
	return c.exts.DeprecatedDirs()
}

// DebugAddrPort returns the addr:port at which Chrome is listening for DevTools connections,
// e.g. "127.0.0.1:38725". This port should not be accessed from outside of this package,
// but it is exposed so that the port's owner can be easily identified.
func (c *Chrome) DebugAddrPort() string {
	return c.sess.DebugAddrPort()
}

// New restarts the ui job, tells Chrome to enable testing, and (by default) logs in.
// The NoLogin option can be passed to avoid logging in.
func New(ctx context.Context, opts ...Option) (c *Chrome, retErr error) {
	if locked {
		panic("Cannot create Chrome instance while precondition is being used")
	}

	ctx, st := timing.Start(ctx, "chrome_new")
	defer st.End()

	cfg, err := config.NewConfig(opts)
	if err != nil {
		return nil, err
	}

	// Cap the timeout to be certain length depending on the login mode. Sometimes
	// chrome.New may fail and get stuck on an unexpected screen. Without timeout,
	// it simply runs out the entire timeout. See https://crbug.com/1078873.
	timeout := LoginTimeout
	if cfg.LoginMode == config.GAIALogin {
		timeout = gaiaLoginTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := checkSoftwareDeps(ctx); err != nil {
		return nil, err
	}

	if err := checkStateful(); err != nil {
		return nil, err
	}

	// Perform an early high-level check of cryptohomed to avoid
	// less-descriptive errors later if it's broken.
	if cfg.LoginMode != config.NoLogin {
		if err := cryptohome.CheckService(ctx); err != nil {
			// Log problems in cryptohomed's dependencies.
			for _, e := range cryptohome.CheckDeps(ctx) {
				testing.ContextLog(ctx, "Potential cryptohome issue: ", e)
			}
			return nil, errors.Wrap(err, "failed to check cryptohome service")
		}
	}

	exts, err := extension.PrepareExtensions(cfg.ExtraExtDirs, cfg.SigninExtKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare extensions")
	}
	defer func() {
		if retErr != nil {
			exts.RemoveAll()
		}
	}()

	if err := restartChromeForTesting(ctx, cfg, exts); err != nil {
		return nil, errors.Wrap(err, "failed to restart chrome for testing")
	}

	agg := jslog.NewAggregator()
	defer func() {
		if retErr != nil {
			agg.Close()
		}
	}()

	sess, err := driver.NewSession(ctx, cdputil.DebuggingPortPath, cdputil.WaitPort, agg)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to establish connection to Chrome Debuggin Protocol with debugging port path=%q", cdputil.DebuggingPortPath)
	}
	defer func() {
		if retErr != nil {
			sess.Close(ctx)
		}
	}()

	if cfg.LoginMode != config.NoLogin && !cfg.KeepState {
		if err := cryptohome.RemoveUserDir(ctx, cfg.NormalizedUser); err != nil {
			return nil, errors.Wrapf(err, "failed to remove cryptohome user directory for %s", cfg.NormalizedUser)
		}
	}

	loginPending := false
	if cfg.DeferLogin {
		loginPending = true
	} else {
		err := login.LogIn(ctx, cfg, sess)
		if err == login.ErrNeedNewSession {
			// Restart session.
			newSess, err := driver.NewSession(ctx, cdputil.DebuggingPortPath, cdputil.WaitPort, agg)
			if err != nil {
				return nil, err
			}
			sess.Close(ctx)
			sess = newSess
		} else if err != nil {
			return nil, err
		}
	}

	// VK uses different extension instance in login profile and user profile.
	// BackgroundConn will wait until the background connection is unique.
	if cfg.VKEnabled {
		// Background target from login persists for a few seconds, causing 2 background targets.
		// Polling until connected to the unique target.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			bconn, err := sess.NewConnForTarget(ctx, MatchTargetURL(vkBackgroundPageURL))
			if err != nil {
				return err
			}
			bconn.Close()
			return nil
		}, &testing.PollOptions{Timeout: 60 * time.Second, Interval: 1 * time.Second}); err != nil {
			return nil, errors.Wrap(err, "failed to wait for unique virtual keyboard background target")
		}
	}

	return &Chrome{
		cfg:          *cfg,
		exts:         exts,
		agg:          agg,
		sess:         sess,
		loginPending: loginPending,
	}, nil
}

// checkSoftwareDeps ensures the current test declares necessary software dependencies.
func checkSoftwareDeps(ctx context.Context) error {
	deps, ok := testing.ContextSoftwareDeps(ctx)
	if !ok {
		// Test info can be unavailable in unit tests.
		return nil
	}

	const needed = "chrome"
	for _, dep := range deps {
		if dep == needed {
			return nil
		}
	}
	return errors.Errorf("test must declare %q software dependency", needed)
}

// checkStateful ensures that the stateful partition is writable.
// This check help debugging in somewhat popular case where disk is physically broken.
// TODO(crbug.com/1047105): Consider moving this check to pre-test hooks if it turns out to be useful.
func checkStateful() error {
	for _, dir := range []string{
		"/mnt/stateful_partition",
		"/mnt/stateful_partition/encrypted",
	} {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue // some dirs may not exist (e.g. on moblab)
		} else if err != nil {
			return errors.Wrapf(err, "failed to stat %s", dir)
		}
		fp := filepath.Join(dir, ".tast.check-disk")
		if err := ioutil.WriteFile(fp, nil, 0600); err != nil {
			return errors.Wrapf(err, "%s is not writable", dir)
		}
		if err := os.Remove(fp); err != nil {
			return errors.Wrapf(err, "%s is not writable", dir)
		}
	}
	return nil
}

// Close disconnects from Chrome and cleans up standard extensions.
// To avoid delays between tests, the ui job (and by extension, Chrome) is not restarted,
// so the current user (if any) remains logged in.
func (c *Chrome) Close(ctx context.Context) error {
	if locked {
		panic("Do not call Close while precondition is being used")
	}

	var firstErr error
	if c.sess != nil {
		firstErr = c.sess.Close(ctx)
	}

	if c.exts != nil {
		c.exts.RemoveAll()
	}

	if dir, ok := testing.ContextOutDir(ctx); ok {
		c.agg.Save(filepath.Join(dir, "jslog.txt"))
	}
	c.agg.Close()

	// As the chronos home directory is cleared during chrome.New(), we
	// should manually move these crashes from the user crash directory to
	// the system crash directory.
	if err := moveUserCrashDumps(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// ResetState attempts to reset Chrome's state (e.g. by closing all pages).
// Tests typically do not need to call this; it is exposed primarily for other packages.
func (c *Chrome) ResetState(ctx context.Context) error {
	testing.ContextLog(ctx, "Resetting Chrome's state")
	ctx, st := timing.Start(ctx, "reset_chrome")
	defer st.End()

	// Try to close all "normal" pages and apps.
	targetFilter := func(t *Target) bool {
		return t.Type == "page" || t.Type == "app"
	}
	targets, err := c.FindTargets(ctx, targetFilter)
	if err != nil {
		return errors.Wrap(err, "failed to get targets")
	}
	var closingTargets []*Target
	if len(targets) > 0 {
		testing.ContextLogf(ctx, "Closing %d target(s)", len(targets))
		for _, t := range targets {
			if err := c.CloseTarget(ctx, t.TargetID); err != nil {
				testing.ContextLogf(ctx, "Failed to close %v: %v", t.URL, err)
			} else {
				// Record all targets that have promised to close
				closingTargets = append(closingTargets, t)
			}
		}
	}
	// Wait for the targets to finish closing
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		targets, err := c.FindTargets(ctx, targetFilter)
		if err != nil {
			return errors.Wrap(err, "failed to get targets")
		}
		var stillClosingCount int
		for _, ct := range closingTargets {
			for _, t := range targets {
				if ct.TargetID == t.TargetID {
					stillClosingCount++
					break
				}
			}
		}
		if stillClosingCount > 0 {
			return errors.Errorf("%d target(s) still open", stillClosingCount)
		}
		return nil
	}, &testing.PollOptions{Interval: 10 * time.Millisecond, Timeout: time.Minute}); err != nil {
		testing.ContextLog(ctx, "Not all targets finished closing: ", err)
	}

	// If the test case started the tracing but somehow StopTracing isn't called,
	// the tracing should be stopped in ResetState.
	if c.sess.TracingStarted() {
		// As noted in the comment of c.StartTracing, the tracingStarted flag is
		// marked before actually StartTracing request is sent because
		// StartTracing's failure doesn't necessarily mean that tracing isn't
		// started. So at this point, c.StopTracing may fail if StartTracing failed
		// and tracing actually didn't start. Because of that, StopTracing's error
		// wouldn't cause an error of ResetState, but simply reporting the error
		// message.
		if _, err := c.sess.StopTracing(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to stop tracing: ", err)
		}
	}

	tconn, err := c.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get test API connection")
	}

	// Free all remote JS objects in the test extension.
	if err := driver.PrivateReleaseAllObjects(ctx, tconn.Conn); err != nil {
		return errors.Wrap(err, "failed to free tast remote JS object group")
	}

	if c.cfg.VKEnabled {
		// Calling the method directly to avoid vkb/chrome circular imports.
		if err := tconn.EvalPromise(ctx, "tast.promisify(chrome.inputMethodPrivate.hideInputView)()", nil); err != nil {
			return errors.Wrap(err, "failed to hide virtual keyboard")
		}

		// Waiting until virtual keyboard disappears from a11y tree.
		var isVKShown bool
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := tconn.Eval(ctx, `
				tast.promisify(chrome.automation.getDesktop)().then(
					root => {return !!(root.find({role: 'rootWebArea', name: 'Chrome OS Virtual Keyboard'}))}
				)`, &isVKShown); err != nil {
				return errors.Wrap(err, "failed to hide virtual keyboard")
			}
			if isVKShown {
				return errors.New("virtual keyboard is still visible")
			}
			return nil
		}, &testing.PollOptions{Interval: 3 * time.Second, Timeout: 30 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait for virtual keyboard to be invisible")
		}
	}

	// Release the mouse buttons in case a test left them pressed. If a button
	// is already released, releasing it is a no-op. Call the method directly to
	// avoid chrome/mouse circular imports.
	// TODO(crbug.com/1096647): Log when a mouse button is pressed.
	for _, button := range []string{"Left", "Right", "Middle"} {
		if err := tconn.Eval(ctx, fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.mouseRelease)(%q)`, button), nil); err != nil {
			return errors.Wrapf(err, "failed to release %s mouse button", button)
		}
	}

	// Clear all notifications in case a test generated some but did not close them.
	if err := tconn.Eval(ctx, "tast.promisify(chrome.autotestPrivate.removeAllNotifications)()", nil); err != nil {
		return errors.Wrap(err, "failed to clear notifications")
	}

	// Disable the automation feature. Otherwise, automation tree updates and
	// events will come to the test API, and sometimes it causes significant
	// performance drawback on low-end devices. See: https://crbug.com/1096719.
	if err := tconn.ResetAutomation(ctx); err != nil {
		return errors.Wrap(err, "failed to reset the automation feature")
	}

	return nil
}

// restartChromeForTesting restarts the ui job, asks session_manager to enable Chrome testing,
// and waits for Chrome to listen on its debugging port.
func restartChromeForTesting(ctx context.Context, cfg *config.Config, exts *extension.Files) error {
	ctx, st := timing.Start(ctx, "restart")
	defer st.End()

	if err := restartSession(ctx, cfg); err != nil {
		// Timeout is often caused by TPM slowness. Save minidumps of related processes.
		if dir, ok := testing.ContextOutDir(ctx); ok {
			minidump.SaveWithoutCrash(ctx, dir,
				minidump.MatchByName("chapsd", "cryptohome", "cryptohomed", "session_manager", "tcsd"))
		}
		return err
	}

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		return err
	}

	// Remove the file where Chrome will write its debugging port after it's restarted.
	os.Remove(cdputil.DebuggingPortPath)

	testing.ContextLog(ctx, "Asking session_manager to enable Chrome testing")
	args := []string{
		"--remote-debugging-port=0",                  // Let Chrome choose its own debugging port.
		"--disable-logging-redirect",                 // Disable redirection of Chrome logging into cryptohome.
		"--ash-disable-system-sounds",                // Disable system startup sound.
		"--autoplay-policy=no-user-gesture-required", // Allow media autoplay.
		"--enable-experimental-extension-apis",       // Allow Chrome to use the Chrome Automation API.
		"--redirect-libassistant-logging",            // Redirect libassistant logging to /var/log/chrome/.
		"--no-startup-window",                        // Do not start up chrome://newtab by default to avoid unexpected patterns(doodle etc.)
		"--no-first-run",                             // Prevent showing up offer pages, e.g. google.com/chromebooks.
		"--cros-region=" + cfg.Region,                // Force the region.
		"--cros-regions-mode=hide",                   // Ignore default values in VPD.
		"--enable-oobe-test-api",                     // Enable OOBE helper functions for authentication.
		"--disable-hid-detection-on-oobe",            // Skip OOBE check for keyboard/mouse on chromeboxes/chromebases.
	}
	if cfg.Enroll {
		args = append(args, "--disable-policy-key-verification") // Remove policy key verification for fake enrollment
	}

	if cfg.SkipOOBEAfterLogin {
		args = append(args, "--oobe-skip-postlogin")
	}

	if !cfg.InstallWebApp {
		args = append(args, "--disable-features=DefaultWebAppInstallation")
	}

	if cfg.VKEnabled {
		args = append(args, "--enable-virtual-keyboard")
	}

	// Enable verbose logging on some enrollment related files.
	if cfg.EnableLoginVerboseLogs {
		args = append(args,
			"--vmodule="+strings.Join([]string{
				"*auto_enrollment_check_screen*=1",
				"*enrollment_screen*=1",
				"*login_display_host_common*=1",
				"*wizard_controller*=1",
				"*auto_enrollment_controller*=1"}, ","))
	}

	if cfg.LoginMode != config.GAIALogin {
		args = append(args, "--disable-gaia-services")
	}
	args = append(args, exts.ChromeArgs()...)
	if cfg.PolicyEnabled {
		args = append(args, "--profile-requires-policy=true")
	} else {
		args = append(args, "--profile-requires-policy=false")
	}
	if cfg.DMSAddr != "" {
		args = append(args, "--device-management-url="+cfg.DMSAddr)
	}
	switch cfg.ARCMode {
	case config.ARCDisabled:
		// Make sure ARC is never enabled.
		args = append(args, "--arc-availability=none")
	case config.ARCEnabled:
		args = append(args,
			// Disable ARC opt-in verification to test ARC with mock GAIA accounts.
			"--disable-arc-opt-in-verification",
			// Always start ARC to avoid unnecessarily stopping mini containers.
			"--arc-start-mode=always-start-with-no-play-store")
	case config.ARCSupported:
		// Allow ARC being enabled on the device to test ARC with real gaia accounts.
		args = append(args, "--arc-availability=officially-supported")
	}
	if cfg.ARCMode == config.ARCEnabled || cfg.ARCMode == config.ARCSupported {
		args = append(args,
			// Do not sync the locale with ARC.
			"--arc-disable-locale-sync",
			// Do not update Play Store automatically.
			"--arc-play-store-auto-update=off",
			// Make 1 Android pixel always match 1 Chrome devicePixel.
			"--force-remote-shell-scale=")
		if !cfg.RestrictARCCPU {
			args = append(args,
				// Disable CPU restrictions to let tests run faster
				"--disable-arc-cpu-restriction")
		}
	}

	if len(cfg.EnableFeatures) != 0 {
		args = append(args, "--enable-features="+strings.Join(cfg.EnableFeatures, ","))
	}

	if len(cfg.DisableFeatures) != 0 {
		args = append(args, "--disable-features="+strings.Join(cfg.DisableFeatures, ","))
	}

	args = append(args, cfg.ExtraArgs...)
	var envVars []string
	if cfg.BreakpadTestMode {
		envVars = append(envVars,
			"CHROME_HEADLESS=",
			"BREAKPAD_DUMP_LOCATION=/home/chronos/crash") // Write crash dumps outside cryptohome.
	}

	// Wait for a browser to start since session_manager can take a while to start it.
	var oldPID int
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		oldPID, err = GetRootPID()
		return err
	}, nil); err != nil {
		return errors.Wrap(err, "failed to find the browser process")
	}

	if _, err = sm.EnableChromeTesting(ctx, true, args, envVars); err != nil {
		return err
	}

	// Wait for a new browser to appear.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		newPID, err := GetRootPID()
		if err != nil {
			return err
		}
		if newPID == oldPID {
			return errors.New("Original browser still running")
		}
		return nil
	}, &testing.PollOptions{Interval: 10 * time.Millisecond, Timeout: 10 * time.Second}); err != nil {
		return err
	}
	return nil
}

// restartSession stops the "ui" job, clears policy files and the user's cryptohome if requested,
// and restarts the job.
func restartSession(ctx context.Context, cfg *config.Config) error {
	testing.ContextLog(ctx, "Restarting ui job")
	ctx, st := timing.Start(ctx, "restart_ui")
	defer st.End()

	ctx, cancel := context.WithTimeout(ctx, uiRestartTimeout)
	defer cancel()

	if err := upstart.StopJob(ctx, "ui"); err != nil {
		return err
	}

	if !cfg.KeepState {
		const chronosDir = "/home/chronos"
		// This always fails because /home/chronos is a mount point, but all files
		// under the directory should be removed.
		os.RemoveAll(chronosDir)
		fis, err := ioutil.ReadDir(chronosDir)
		if err != nil {
			return err
		}
		// Retry cleanup of remaining files. Don't fail if removal reports an error.
		for _, left := range fis {
			if err := os.RemoveAll(filepath.Join(chronosDir, left.Name())); err != nil {
				testing.ContextLogf(ctx, "Failed to clear %s; failed to remove %q: %v", chronosDir, left.Name(), err)
			} else {
				testing.ContextLogf(ctx, "Failed to clear %s; %q needed repeated removal", chronosDir, left.Name())
			}
		}

		// Delete files from shadow directory.
		const shadowDir = "/home/.shadow"
		shadowFiles, err := ioutil.ReadDir(shadowDir)
		if err != nil {
			return errors.Wrapf(err, "failed to read directory %q", shadowDir)
		}
		for _, file := range shadowFiles {
			if !file.IsDir() {
				continue
			}
			// Only look for chronos file with names matching u-*.
			chronosName := filepath.Join(chronosDir, "u-"+file.Name())
			shadowName := filepath.Join(shadowDir, file.Name())
			// Remove the shadow directory if it does not have a corresponding chronos directory.
			if _, err := os.Stat(chronosName); err != nil && os.IsNotExist(err) {
				if err := os.RemoveAll(shadowName); err != nil {
					testing.ContextLogf(ctx, "Failed to remove %q: %v", shadowName, err)
				}
			}
		}

		// Delete policy files to clear the device's ownership state since the account
		// whose cryptohome we'll delete may be the owner: http://cbug.com/897278
		if err := session.ClearDeviceOwnership(ctx); err != nil {
			return err
		}
	}
	return upstart.EnsureJobRunning(ctx, "ui")
}

// NewConn creates a new Chrome renderer and returns a connection to it.
// If url is empty, an empty page (about:blank) is opened. Otherwise, the page
// from the specified URL is opened. You can assume that the page loading has
// been finished when this function returns.
func (c *Chrome) NewConn(ctx context.Context, url string, opts ...cdputil.CreateTargetOption) (*Conn, error) {
	return c.sess.NewConn(ctx, url, opts...)
}

// Target describes a DevTools target.
type Target = driver.Target

// TargetID is an ID assigned to a DevTools target.
type TargetID = driver.TargetID

// TargetMatcher is a caller-provided function that matches targets with specific characteristics.
type TargetMatcher = driver.TargetMatcher

// MatchTargetID returns a TargetMatcher that matches targets with the supplied ID.
func MatchTargetID(id TargetID) TargetMatcher {
	return driver.MatchTargetID(id)
}

// MatchTargetURL returns a TargetMatcher that matches targets with the supplied URL.
func MatchTargetURL(url string) TargetMatcher {
	return driver.MatchTargetURL(url)
}

// NewConnForTarget iterates through all available targets and returns a connection to the
// first one that is matched by tm. It polls until the target is found or ctx's deadline expires.
// An error is returned if no target is found, tm matches multiple targets, or the connection cannot
// be established.
//
//	f := func(t *Target) bool { return t.URL == "http://example.net/" }
//	conn, err := cr.NewConnForTarget(ctx, f)
func (c *Chrome) NewConnForTarget(ctx context.Context, tm TargetMatcher) (*Conn, error) {
	return c.sess.NewConnForTarget(ctx, tm)
}

// FindTargets returns the info about Targets, which satisfies the given cond condition.
func (c *Chrome) FindTargets(ctx context.Context, tm TargetMatcher) ([]*Target, error) {
	return c.sess.FindTargets(ctx, tm)
}

// CloseTarget closes the target identified by the given id.
func (c *Chrome) CloseTarget(ctx context.Context, id TargetID) error {
	return c.sess.CloseTarget(ctx, id)
}

// ExtensionBackgroundPageURL returns the URL to the background page for
// the extension with the supplied ID.
func ExtensionBackgroundPageURL(extID string) string {
	return extension.BackgroundPageURL(extID)
}

// TestConn is a connection to the Tast test extension's background page.
// cf) crbug.com/1043590
type TestConn = driver.TestConn

// TestAPIConn returns a shared connection to the test API extension's
// background page (which can be used to access various APIs). The connection is
// lazily created, and this function will block until the extension is loaded or
// ctx's deadline is reached. The caller should not close the returned
// connection; it will be closed automatically by Close.
func (c *Chrome) TestAPIConn(ctx context.Context) (*TestConn, error) {
	return c.sess.TestAPIConn(ctx)
}

// SigninProfileTestAPIConn is the same as TestAPIConn, but for the signin
// profile test extension.
func (c *Chrome) SigninProfileTestAPIConn(ctx context.Context) (*TestConn, error) {
	return c.sess.SigninProfileTestAPIConn(ctx)
}

// Responded performs basic checks to verify that Chrome has not crashed.
func (c *Chrome) Responded(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "check_chrome")
	defer st.End()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := c.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	result := false
	if err = conn.Eval(ctx, "true", &result); err != nil {
		return err
	}
	if !result {
		return errors.New("eval 'true' returned false")
	}
	return nil
}

// WaitForOOBEConnection waits for that the OOBE page is shown, then returns
// a connection to the page. The caller must close the returned connection.
func (c *Chrome) WaitForOOBEConnection(ctx context.Context) (*Conn, error) {
	return login.WaitForOOBEConnection(ctx, c.sess)
}

// ContinueLogin continues login deferred by DeferLogin option. It is an error to call
// this method when DeferLogin option was not passed to New.
func (c *Chrome) ContinueLogin(ctx context.Context) error {
	if !c.loginPending {
		return errors.New("ContinueLogin can be called once after DeferLogin option is used")
	}
	c.loginPending = false

	err := login.LogIn(ctx, &c.cfg, c.sess)
	if err == login.ErrNeedNewSession {
		// Restart session.
		newSess, err := driver.NewSession(ctx, cdputil.DebuggingPortPath, cdputil.WaitPort, c.agg)
		if err != nil {
			return err
		}
		c.sess.Close(ctx)
		c.sess = newSess
	} else if err != nil {
		return err
	}
	return nil
}

// IsTargetAvailable checks if there is any matched target.
func (c *Chrome) IsTargetAvailable(ctx context.Context, tm TargetMatcher) (bool, error) {
	targets, err := c.FindTargets(ctx, tm)
	if err != nil {
		return false, errors.Wrap(err, "failed to get targets")
	}
	return len(targets) != 0, nil
}

// StartTracing starts trace events collection for the selected categories. Android
// categories must be prefixed with "disabled-by-default-android ", e.g. for the
// gfx category, use "disabled-by-default-android gfx", including the space.
// Note: StopTracing should be called even if StartTracing returns an error.
// Sometimes, the request to start tracing reaches the browser process, but there
// is a timeout while waiting for the reply.
func (c *Chrome) StartTracing(ctx context.Context, categories []string) error {
	return c.sess.StartTracing(ctx, categories)
}

// StopTracing stops trace collection and returns the collected trace events.
func (c *Chrome) StopTracing(ctx context.Context) (*trace.Trace, error) {
	return c.sess.StopTracing(ctx)
}

// SaveTraceToFile marshals the given trace into a binary protobuf and saves it
// to a gzip archive at the specified path.
func SaveTraceToFile(ctx context.Context, trace *trace.Trace, path string) error {
	data, err := proto.Marshal(trace)
	if err != nil {
		return errors.Wrap(err, "could not marshal trace to binary")
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return errors.Wrap(err, "could not open file")
	}
	defer func() {
		if err := file.Close(); err != nil {
			testing.ContextLog(ctx, "Failed to close file: ", err)
		}
	}()

	writer := gzip.NewWriter(file)
	defer func() {
		if err := writer.Close(); err != nil {
			testing.ContextLog(ctx, "Failed to close gzip writer: ", err)
		}
	}()

	if _, err := writer.Write(data); err != nil {
		return errors.Wrap(err, "could not write the data")
	}

	if err := writer.Flush(); err != nil {
		return errors.Wrap(err, "could not flush the gzip writer")
	}

	return nil
}

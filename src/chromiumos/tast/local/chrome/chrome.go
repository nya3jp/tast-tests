// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chrome implements a library used for communication with Chrome.
package chrome

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/minidump"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"

	"github.com/godbus/dbus"

	"github.com/mafredri/cdp/devtool"
)

const (
	chromeUser        = "chronos"                          // Chrome Unix username
	debuggingPortPath = "/home/chronos/DevToolsActivePort" // file where Chrome writes debugging port
	crashDir          = "/home/chronos/crash"              // directory to write crashes to

	defaultUser   = "testuser@gmail.com"
	defaultPass   = "testpass"
	defaultGaiaID = "gaia-id"

	oobePrefix = "chrome://oobe"

	// ui-post-stop can sometimes block for an extended period of time
	// waiting for "cryptohome --action=pkcs11_terminate" to finish: https://crbug.com/860519
	uiRestartTimeout = 90 * time.Second
)

// Use a low polling interval while waiting for conditions during login, as this code is shared by many tests.
var loginPollOpts *testing.PollOptions = &testing.PollOptions{Interval: 10 * time.Millisecond}

// arcMode describes the mode that ARC should be put into.
type arcMode int

const (
	arcDisabled arcMode = iota
	arcEnabled
)

// option is a self-referential function can be used to configure Chrome.
// See https://commandcenter.blogspot.com.au/2014/01/self-referential-functions-and-design.html
// for details about this pattern.
type option func(c *Chrome)

// Auth returns an option that can be passed to New to configure the login credentials used by Chrome.
func Auth(user, pass, gaiaID string) option {
	return func(c *Chrome) {
		c.user = user
		c.pass = pass
		c.gaiaID = gaiaID
	}
}

// KeepCryptohome returns an option that can be passed to New to preserve the user's existing
// cryptohome (if any) instead of wiping it before logging in.
func KeepCryptohome() option {
	return func(c *Chrome) {
		c.keepCryptohome = true
	}
}

// NoLogin returns an option that can be passed to New to avoid logging in.
// Chrome is still restarted with testing-friendly behavior.
func NoLogin() option {
	return func(c *Chrome) {
		c.shouldLogIn = false
	}
}

// ARCEnabled returns an option that can be passed to New to enable ARC (without Play Store)
// for the user session.
func ARCEnabled() option {
	return func(c *Chrome) {
		c.arcMode = arcEnabled
	}
}

// ExtraArgs returns an option that can be passed to New to append additional arguments to Chrome's command line.
func ExtraArgs(args []string) option {
	return func(c *Chrome) {
		c.extraArgs = append(c.extraArgs, args...)
	}
}

// Chrome interacts with the currently-running Chrome instance via the
// Chrome DevTools protocol (https://chromedevtools.github.io/devtools-protocol/).
type Chrome struct {
	devt               *devtool.DevTools
	user, pass, gaiaID string // login credentials
	keepCryptohome     bool
	shouldLogIn        bool
	arcMode            arcMode
	extraArgs          []string

	extsDir     string // contains subdirs with unpacked extensions
	testExtId   string // ID for extension exposing APIs
	testExtConn *Conn  // connection to extension exposing APIs

	watcher *browserWatcher // tries to catch Chrome restarts
}

// User returns the username that was used to log in to Chrome.
func (c *Chrome) User() string { return c.user }

// New restarts the ui job, tells Chrome to enable testing, and (by default) logs in.
// The NoLogin option can be passed to avoid logging in.
func New(ctx context.Context, opts ...option) (*Chrome, error) {
	c := &Chrome{
		user:           defaultUser,
		pass:           defaultPass,
		gaiaID:         defaultGaiaID,
		keepCryptohome: false,
		shouldLogIn:    true,
		watcher:        newBrowserWatcher(),
	}
	for _, opt := range opts {
		opt(c)
	}

	// Clean up the partially-initialized object on error.
	toClose := c
	defer func() {
		if toClose != nil {
			toClose.Close(ctx)
		}
	}()

	var port int
	var err error
	if err = c.writeExtensions(); err != nil {
		return nil, err
	}
	if port, err = c.restartChromeForTesting(ctx); err != nil {
		return nil, err
	}
	c.devt = devtool.New(fmt.Sprintf("http://127.0.0.1:%d", port))

	if !c.keepCryptohome {
		if err = cryptohome.RemoveUserDir(ctx, c.user); err != nil {
			return nil, err
		}
	}

	if c.shouldLogIn {
		if err = c.logIn(ctx); err != nil {
			return nil, err
		}
	}

	toClose = nil
	return c, nil
}

// Close disconnects from Chrome and cleans up standard extensions.
func (c *Chrome) Close(ctx context.Context) error {
	// TODO(derat): Decide if it's okay to skip restarting the ui job here.
	// We're leaving the system in a logged-in state, but at the same time,
	// restartChromeForTesting restarts the job too, and we can shave a few
	// seconds off each UI test by not doing it again here... ¯\_(ツ)_/¯
	if c.testExtConn != nil {
		c.testExtConn.Close()
	}
	if len(c.extsDir) > 0 {
		os.RemoveAll(c.extsDir)
	}
	c.watcher.close()
	return nil
}

// chromeErr returns c.watcher.err() if non-nil or orig otherwise. This is useful for
// replacing "context deadline exceeded" errors that can occur when Chrome is crashing
// with more-descriptive ones.
func (c *Chrome) chromeErr(orig error) error {
	if werr := c.watcher.err(); werr != nil {
		return werr
	}
	return orig
}

// writeExtensions creates a temporary directory and writes standard extensions needed by
// tests to it.
func (c *Chrome) writeExtensions() error {
	var err error
	if c.extsDir, err = ioutil.TempDir("", "tast_chrome_extensions."); err != nil {
		return err
	}
	if c.testExtId, err = writeTestExtension(
		filepath.Join(c.extsDir, "test_api_extension")); err != nil {
		return err
	}
	// Chrome hangs with a nonsensical "Extension error: Failed to load extension
	// from: . Manifest file is missing or unreadable." error if an extension directory
	// is owned by another user.
	return chownContents(c.extsDir, chromeUser)
}

// readDebuggingPort returns the port number from the first line of p, a file
// written by Chrome when --remote-debugging-port=0 is passed.
func readDebuggingPort(p string) (int, error) {
	b, err := ioutil.ReadFile(p)
	if err != nil {
		return -1, err
	}
	lines := strings.Split(string(b), "\n")
	port, err := strconv.ParseInt(lines[0], 10, 32)
	return int(port), err
}

// restartChromeForTesting restarts the ui job, asks session_manager to enable Chrome testing,
// and waits for Chrome to listen on its debugging port.
func (c *Chrome) restartChromeForTesting(ctx context.Context) (port int, err error) {
	rctx, cancel := context.WithTimeout(ctx, uiRestartTimeout)
	defer cancel()

	testing.ContextLog(ctx, "Restarting ui job")
	if err := upstart.RestartJob(rctx, "ui"); err != nil {
		// Timeout is often caused by TPM slowness. Save minidumps of related processes.
		minidump.SaveWithoutCrash(
			ctx,
			testing.ContextOutDir(ctx),
			minidump.MatchByName("chapsd", "cryptohome", "cryptohomed", "session_manager", "tcsd"))
		return -1, err
	}

	bus, err := dbus.SystemBus()
	if err != nil {
		return -1, fmt.Errorf("failed to connect to system bus: %v", err)
	}

	testing.ContextLogf(ctx, "Waiting for %s D-Bus service", dbusutil.SessionManagerName)
	if err = dbusutil.WaitForService(ctx, bus, dbusutil.SessionManagerName); err != nil {
		return -1, fmt.Errorf("failed to wait for %s: %v", dbusutil.SessionManagerName, err)
	}

	extDirs, err := getExtensionDirs(c.extsDir)
	if err != nil {
		return -1, err
	}

	// Remove the file where Chrome will write its debugging port after it's restarted.
	os.Remove(debuggingPortPath)

	testing.ContextLog(ctx, "Asking session_manager to enable Chrome testing")
	obj := bus.Object(dbusutil.SessionManagerName, dbusutil.SessionManagerPath)
	method := fmt.Sprintf("%s.%s", dbusutil.SessionManagerInterface, "EnableChromeTesting")
	args := []string{
		"--remote-debugging-port=0",                  // Let Chrome choose its own debugging port.
		"--disable-logging-redirect",                 // Disable redirection of Chrome logging into cryptohome.
		"--ash-disable-system-sounds",                // Disable system startup sound.
		"--oobe-skip-postlogin",                      // Skip post-login screens.
		"--disable-gaia-services",                    // TODO(derat): Reconsider this if/when supporting GAIA login.
		"--autoplay-policy=no-user-gesture-required", // Allow media autoplay.
		"--enable-experimental-extension-apis",       // Allow Chrome to use the Chrome Automation API.
	}
	if len(extDirs) > 0 {
		args = append(args, "--load-extension="+strings.Join(extDirs, ","))
	}
	switch c.arcMode {
	case arcDisabled:
		// Make sure ARC is never enabled.
		args = append(args, "--arc-availability=none")
	case arcEnabled:
		args = append(args,
			// Disable ARC opt-in verification to test ARC with mock GAIA accounts.
			"--disable-arc-opt-in-verification",
			// Always start ARC to avoid unnecessarily stopping mini containers.
			"--arc-start-mode=always-start-with-no-play-store")
	}
	args = append(args, c.extraArgs...)
	envVars := []string{
		"CHROME_HEADLESS=",                   // Force crash dumping.
		"BREAKPAD_DUMP_LOCATION=" + crashDir, // Write crash dumps outside cryptohome.
	}
	if call := obj.CallWithContext(ctx, method, 0, true, args, envVars); call.Err != nil {
		return -1, call.Err
	}

	// The original browser process should be gone now, so start watching for the new one.
	c.watcher.start()

	testing.ContextLog(ctx, "Waiting for Chrome to write its debugging port to ", debuggingPortPath)
	if err = testing.Poll(ctx, func(context.Context) error {
		port, err = readDebuggingPort(debuggingPortPath)
		return err
	}, loginPollOpts); err != nil {
		return -1, fmt.Errorf("failed to read Chrome debugging port: %v", c.chromeErr(err))
	}

	return port, nil
}

// NewConn creates a new Chrome renderer and returns a connection to it.
func (c *Chrome) NewConn(ctx context.Context, url string) (*Conn, error) {
	var t *devtool.Target
	var err error
	if url == "" {
		testing.ContextLog(ctx, "Creating new blank page")
		t, err = c.devt.Create(ctx)
	} else {
		testing.ContextLog(ctx, "Creating new page with URL ", url)
		t, err = c.devt.CreateURL(ctx, url)
	}
	if err != nil {
		return nil, err
	}
	return newConn(ctx, t.WebSocketDebuggerURL, c.chromeErr)
}

// Target contains information about an available debugging target to which a connection can be established.
type Target struct {
	// URL contains the URL of the resource currently loaded by the target.
	URL string
}

func newTarget(t *devtool.Target) *Target {
	return &Target{URL: t.URL}
}

// TargetMatcher is a caller-provided function that matches targets with specific characteristics.
type TargetMatcher func(t *Target) bool

// NewConnForTarget iterates through all available targets and returns a connection to the
// first one that is matched by tm. It polls until the target is found or ctx's deadline expires.
// An error is returned if no target is found, tm matches multiple targets, or the connection cannot
// be established.
//
//	f := func(t *Target) bool { return t.URL = "http://example.net/" }
//	conn, err := cr.NewConnForTarget(ctx, f)
func (c *Chrome) NewConnForTarget(ctx context.Context, tm TargetMatcher) (*Conn, error) {
	var errNoMatch = errors.New("no targets matched")

	matchAll := func(t *devtool.Target) bool { return true }

	var all, matched []*devtool.Target
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		all, err = c.getDevtoolTargets(ctx, matchAll)
		if err != nil {
			return c.chromeErr(err)
		}
		matched = []*devtool.Target{}
		for _, t := range all {
			if tm(newTarget(t)) {
				matched = append(matched, t)
			}
		}
		if len(matched) == 0 {
			return errNoMatch
		}
		return nil
	}, loginPollOpts); err != nil && err != errNoMatch {
		return nil, err
	}

	if len(matched) != 1 {
		testing.ContextLogf(ctx, "%d targets matched while unique match was expected. Existing targets:", len(matched))
		for _, t := range all {
			testing.ContextLogf(ctx, "  %+v", newTarget(t))
		}
		return nil, fmt.Errorf("%d targets found", len(matched))
	}
	return newConn(ctx, matched[0].WebSocketDebuggerURL, c.chromeErr)
}

// TestAPIConn returns a shared connection to the test API extension's
// background page (which can be used to access various APIs). The connection is
// lazily created, and this function will block until the extension is loaded or
// ctx's deadline is reached. The caller should not close the returned
// connection; it will be closed automatically by Close.
func (c *Chrome) TestAPIConn(ctx context.Context) (*Conn, error) {
	if c.testExtConn != nil {
		return c.testExtConn, nil
	}

	extURL := "chrome-extension://" + c.testExtId + "/_generated_background_page.html"
	testing.ContextLog(ctx, "Waiting for test API extension at ", extURL)
	f := func(t *Target) bool { return t.URL == extURL }
	var err error
	if c.testExtConn, err = c.NewConnForTarget(ctx, f); err != nil {
		return nil, err
	}

	// Ensure that we don't attempt to use the extension before its APIs are
	// available: https://crbug.com/789313
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ready := false
		if err := c.testExtConn.Eval(ctx, "'autotestPrivate' in chrome", &ready); err != nil {
			return err
		} else if !ready {
			return errors.New("no autotestPrivate property")
		}
		return nil
	}, loginPollOpts); err != nil {
		return nil, fmt.Errorf("chrome.autotestPrivate unavailable: %v", err)
	}

	testing.ContextLog(ctx, "Test API extension is ready")
	return c.testExtConn, nil
}

// getDevtoolTargets returns all DevTools targets matched by f.
func (c *Chrome) getDevtoolTargets(ctx context.Context, f func(*devtool.Target) bool) ([]*devtool.Target, error) {
	all, err := c.devt.List(ctx)
	if err != nil {
		return nil, err
	}

	matches := make([]*devtool.Target, 0)
	for _, t := range all {
		if f(t) {
			matches = append(matches, t)
		}
	}
	return matches, nil
}

// getFirstOOBETarget returns the first OOBE-related DevTools target that it finds.
// nil is returned if no target is found.
func (c *Chrome) getFirstOOBETarget(ctx context.Context) (*devtool.Target, error) {
	targets, err := c.getDevtoolTargets(ctx, func(t *devtool.Target) bool {
		return strings.HasPrefix(t.URL, oobePrefix)
	})
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, nil
	}
	return targets[0], nil
}

// logIn logs in to a freshly-restarted Chrome instance.
// It waits for the login process to complete before returning.
func (c *Chrome) logIn(ctx context.Context) error {
	testing.ContextLog(ctx, "Finding OOBE DevTools target")
	var target *devtool.Target
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		if target, err = c.getFirstOOBETarget(ctx); err != nil {
			return err
		} else if target == nil {
			return fmt.Errorf("no %s target", oobePrefix)
		}
		return nil
	}, loginPollOpts); err != nil {
		return fmt.Errorf("OOBE target not found: %v", c.chromeErr(err))
	}

	conn, err := newConn(ctx, target.WebSocketDebuggerURL, c.chromeErr)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Cribbed from telemetry/internal/backends/chrome/cros_browser_backend.py in Catapult.
	testing.ContextLog(ctx, "Waiting for OOBE")
	if err = conn.WaitForExpr(ctx, "typeof Oobe == 'function' && Oobe.readyForTesting"); err != nil {
		return fmt.Errorf("OOBE didn't show up (Oobe.readyForTesting not found): %v", c.chromeErr(err))
	}
	missing := true
	if err = conn.Eval(ctx, "Oobe.loginForTesting === undefined", &missing); err != nil {
		return err
	}
	if missing {
		return errors.New("Oobe.loginForTesting API is missing")
	}

	testing.ContextLogf(ctx, "Logging in as user %q", c.user)
	if err = conn.Exec(ctx, fmt.Sprintf("Oobe.loginForTesting('%s', '%s', '%s', false)", c.user, c.pass, c.gaiaID)); err != nil {
		return err
	}

	if err = cryptohome.WaitForUserMount(ctx, c.user); err != nil {
		return err
	}

	// TODO(derat): Probably need to reconnect here if Chrome restarts due to logging in as guest (or flag changes?).

	testing.ContextLog(ctx, "Waiting for OOBE to be dismissed")
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		if t, err := c.getFirstOOBETarget(ctx); err != nil {
			return err
		} else if t != nil {
			return fmt.Errorf("%s target still exists", oobePrefix)
		}
		return nil
	}, loginPollOpts); err != nil {
		return fmt.Errorf("OOBE not dismissed: %v", c.chromeErr(err))
	}
	return nil
}

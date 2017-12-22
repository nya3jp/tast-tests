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

	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"

	"github.com/godbus/dbus"

	"github.com/mafredri/cdp/devtool"
)

// arcMode describes the mode that ARC should be put into.
type arcMode int

const (
	chromeUser        = "chronos"                          // Chrome Unix username
	debuggingPortPath = "/home/chronos/DevToolsActivePort" // file where Chrome writes debugging port
	crashDir          = "/home/chronos/crash"              // directory to write crashes to

	defaultUser   = "testuser@gmail.com"
	defaultPass   = "testpass"
	defaultGaiaID = "gaia-id"

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

// ARCEnabled returns an option that can be passed to New to enable ARC for the user session.
// ARC opt-in verification is bypassed; Android will be usable when New returns.
func ARCEnabled() option {
	return func(c *Chrome) {
		c.arcMode = arcEnabled
	}
}

// KeepCryptohome returns an option that can be passed to New to preserve the user's existing
// cryptohome (if any) instead of wiping it before logging in.
func KeepCryptohome() option {
	return func(c *Chrome) {
		c.keepCryptohome = true
	}
}

// MashEnabled returns an option that can be passed to New to run ash system UI in out-of-process
// mode (https://chromium.googlesource.com/chromium/src/+/master/ash/README.md).
func MashEnabled() option {
	return func(c *Chrome) {
		c.mashEnabled = true
	}
}

// NoLogin returns an option that can be passed to New to avoid logging in.
// Chrome is still restarted with testing-friendly behavior.
func NoLogin() option {
	return func(c *Chrome) {
		c.shouldLogIn = false
	}
}

// Chrome interacts with the currently-running Chrome instance via the
// Chrome DevTools protocol (https://chromedevtools.github.io/devtools-protocol/).
type Chrome struct {
	devt               *devtool.DevTools
	user, pass, gaiaID string // login credentials
	arcMode            arcMode
	keepCryptohome     bool
	mashEnabled        bool
	shouldLogIn        bool

	extsDir     string // contains subdirs with unpacked extensions
	testExtId   string // ID for extension exposing APIs
	testExtConn *Conn  // connection to extension exposing APIs
}

// New restarts the ui job, tells Chrome to enable testing, and (by default) logs in.
// The NoLogin option can be passed to avoid logging in.
func New(ctx context.Context, opts ...option) (*Chrome, error) {
	c := &Chrome{
		user:           defaultUser,
		pass:           defaultPass,
		gaiaID:         defaultGaiaID,
		arcMode:        arcDisabled,
		keepCryptohome: false,
		mashEnabled:    false,
		shouldLogIn:    true,
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
		if err = clearCryptohome(ctx, c.user); err != nil {
			return nil, err
		}
	}

	if c.shouldLogIn {
		if err = c.logIn(ctx); err != nil {
			return nil, err
		}
		if c.arcMode == arcEnabled {
			if err := enablePlayStore(ctx, c); err != nil {
				return nil, fmt.Errorf("failed enabling Play Store: %v", err)
			}
			if err := waitForAndroidBooted(ctx); err != nil {
				return nil, fmt.Errorf("Android didn't boot: %v", err)
			}
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
	return nil
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
	testing.ContextLog(ctx, "Restarting ui job")
	if err := upstart.RestartJob("ui"); err != nil {
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
		"--remote-debugging-port=0",   // Let Chrome choose its own debugging port.
		"--disable-logging-redirect",  // Disable redirection of Chrome logging into cryptohome.
		"--ash-disable-system-sounds", // Disable system startup sound.
		"--oobe-skip-postlogin",       // Skip post-login screens.
		"--disable-gaia-services",     // TODO(derat): Reconsider this if/when supporting GAIA login.
	}
	if len(extDirs) > 0 {
		args = append(args, "--load-extension="+strings.Join(extDirs, ","))
	}
	if c.arcMode == arcEnabled {
		args = append(args, "--disable-arc-opt-in-verification")
	}
	if c.mashEnabled {
		args = append(args, "--mash")
	}
	envVars := []string{
		"CHROME_HEADLESS=",                   // Force crash dumping.
		"BREAKPAD_DUMP_LOCATION=" + crashDir, // Write crash dumps outside cryptohome.
	}
	if call := obj.Call(method, 0, true, args, envVars); call.Err != nil {
		return -1, call.Err
	}

	testing.ContextLog(ctx, "Waiting for Chrome to write its debugging port")
	if err = poll(ctx, func() bool {
		port, err = readDebuggingPort(debuggingPortPath)
		return err == nil
	}); err != nil {
		return -1, fmt.Errorf("failed to read Chrome debugging port from %s: %v", debuggingPortPath, err)
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
	return newConn(ctx, t.WebSocketDebuggerURL)
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

	extUrl := "chrome-extension://" + c.testExtId + "/_generated_background_page.html"
	var target *devtool.Target
	testing.ContextLog(ctx, "Waiting for test API extension at ", extUrl)
	f := func() bool {
		ts, err := c.getDevtoolTargets(ctx, func(t *devtool.Target) bool {
			return t.URL == extUrl
		})
		if err == nil && len(ts) > 0 {
			target = ts[0]
		}
		return target != nil
	}
	if err := poll(ctx, f); err != nil {
		return nil, fmt.Errorf("didn't get target: %v", err)
	}

	var err error
	if c.testExtConn, err = newConn(ctx, target.WebSocketDebuggerURL); err != nil {
		return nil, err
	}

	// Ensure that we don't attempt to use the extension before its APIs are
	// available: https://crbug.com/789313
	if err = poll(ctx, func() bool {
		ready := false
		c.testExtConn.Eval(ctx, "'autotestPrivate' in chrome", &ready)
		return ready
	}); err != nil {
		return nil, fmt.Errorf("chrome.autotestPrivate unavailable: %v", err)
	}

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
		return strings.HasPrefix(t.URL, "chrome://oobe")
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
	if err := poll(ctx, func() bool {
		var e error
		target, e = c.getFirstOOBETarget(ctx)
		return e == nil && target != nil
	}); err != nil {
		return fmt.Errorf("OOBE target not found: %v", err)
	}

	conn, err := newConn(ctx, target.WebSocketDebuggerURL)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Cribbed from telemetry/internal/backends/chrome/cros_browser_backend.py in Catapult.
	testing.ContextLog(ctx, "Waiting for OOBE")
	if err = conn.WaitForExpr(ctx, "typeof Oobe == 'function' && Oobe.readyForTesting"); err != nil {
		return err
	}
	missing := true
	if err = conn.Eval(ctx, "typeof Oobe.loginForTesting == 'undefined'", &missing); err != nil {
		return err
	}
	if missing {
		return errors.New("Oobe.loginForTesting API is missing")
	}

	testing.ContextLogf(ctx, "Logging in as user %q", c.user)
	if err = conn.Exec(ctx, fmt.Sprintf("Oobe.loginForTesting('%s', '%s', '%s', false)", c.user, c.pass, c.gaiaID)); err != nil {
		return err
	}

	if err = waitForCryptohome(ctx, c.user); err != nil {
		return err
	}

	// TODO(derat): Probably need to reconnect here if Chrome restarts due to logging in as guest (or flag changes?).

	testing.ContextLog(ctx, "Waiting for OOBE to be dismissed")
	if err = poll(ctx, func() bool {
		t, err := c.getFirstOOBETarget(ctx)
		return err == nil && t == nil
	}); err != nil {
		return fmt.Errorf("OOBE not dismissed: %v", err)
	}
	return nil
}

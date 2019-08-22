// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chrome implements a library used for communication with Chrome.
package chrome

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/caller"
	"chromiumos/tast/crash"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/cdputil"
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
	LoginTimeout = 60 * time.Second

	chromeUser = "chronos" // Chrome Unix username

	// DefaultUser contains the email address used to log into Chrome when authentication credentials are not supplied.
	DefaultUser = "testuser@gmail.com"

	// DefaultPass contains the password we use to log into the DefaultUser account.
	DefaultPass   = "testpass"
	defaultGaiaID = "gaia-id"

	oobePrefix = "chrome://oobe"

	// ui-post-stop can sometimes block for an extended period of time
	// waiting for "cryptohome --action=pkcs11_terminate" to finish: https://crbug.com/860519
	uiRestartTimeout = 90 * time.Second

	blankURL = "about:blank"
)

// Use a low polling interval while waiting for conditions during login, as this code is shared by many tests.
var loginPollOpts *testing.PollOptions = &testing.PollOptions{Interval: 10 * time.Millisecond}

// locked is set to true while a precondition is active to prevent tests from calling New or Chrome.Close.
var locked = false

// prePackages lists packages containing preconditions that are allowed to call Lock and Unlock.
var prePackages = []string{
	"chromiumos/tast/local/arc",
	"chromiumos/tast/local/chrome",
	"chromiumos/tast/local/crostini",
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

// arcMode describes the mode that ARC should be put into.
type arcMode int

const (
	arcDisabled arcMode = iota
	arcEnabled
	arcSupported // ARC is supported and can be launched by user policy
)

// loginMode describes the user mode for the login.
type loginMode int

const (
	noLogin    loginMode = iota // restart Chrome but don't log in
	fakeLogin                   // fake login with no authentication
	gaiaLogin                   // real network-based login using GAIA backend
	guestLogin                  // sign in as ephemeral guest user
)

// policyMode describes how/if Chrome should get policies.
type policyMode int

const (
	noPolicy    policyMode = iota // do not fetch policies
	fetchPolicy                   // fetch policies like a regular user
)

// option is a self-referential function can be used to configure Chrome.
// See https://commandcenter.blogspot.com.au/2014/01/self-referential-functions-and-design.html
// for details about this pattern.
type option func(c *Chrome)

// Auth returns an option that can be passed to New to configure the login credentials used by Chrome.
// Please do not check in real credentials to public repositories when using this in conjunction with GAIALogin.
func Auth(user, pass, gaiaID string) option {
	return func(c *Chrome) {
		c.user = user
		c.pass = pass
		c.gaiaID = gaiaID
	}
}

// KeepState returns an option that can be passed to New to preserve the state such as
// files under /home/chronos and the user's existing cryptohome (if any) instead of
// wiping them before logging in.
func KeepState() option {
	return func(c *Chrome) { c.keepState = true }
}

// GAIALogin returns an option that can be passed to New to perform a real GAIA-based login rather
// than the default fake login.
func GAIALogin() option {
	return func(c *Chrome) { c.loginMode = gaiaLogin }
}

// NoLogin returns an option that can be passed to New to avoid logging in.
// Chrome is still restarted with testing-friendly behavior.
func NoLogin() option {
	return func(c *Chrome) { c.loginMode = noLogin }
}

// GuestLogin returns an option that can be passed to New to log in as guest
// user.
func GuestLogin() option {
	return func(c *Chrome) {
		c.loginMode = guestLogin
		c.user = cryptohome.GuestUser
	}
}

// Region returns an option that can be passed to New to set the region deciding
// the locale used in the OOBE screen and the user sessions. region is a
// two-letter code such as "us", "fr", or "ja".
func Region(region string) option {
	return func(c *Chrome) {
		c.region = region
	}
}

// FetchPolicy returns an option that can be passed to New to let the device do a policy fetch
// upon login. By default, policies are not fetched.
func FetchPolicy() option {
	return func(c *Chrome) { c.policyMode = fetchPolicy }
}

// ARCEnabled returns an option that can be passed to New to enable ARC (without Play Store)
// for the user session with mock GAIA account.
func ARCEnabled() option {
	return func(c *Chrome) { c.arcMode = arcEnabled }
}

// ARCSupported returns an option that can be passed to New to allow to enable ARC with Play Store gaia opt-in for the user
// session with real GAIA account.
// In this case ARC is not launched by default and is required to be launched by user policy or from UI.
func ARCSupported() option {
	return func(c *Chrome) { c.arcMode = arcSupported }
}

// RestrictARCCPU returns an option that can be passed to New which controls whether
// to let Chrome use CGroups to limit the CPU time of ARC when in the background.
// Most ARC-related tests should not pass this option.
func RestrictARCCPU() option {
	return func(c *Chrome) { c.restrictARCCPU = true }
}

// CrashNormalMode tells the crash handling system to act like it would on a
// real device. If this option is not used, the Chrome instances created by this package
// will skip calling crash_reporter and write any dumps into /home/chronos/crash directly
// from breakpad. This option restores the normal behavior of calling crash_reporter.
func CrashNormalMode() option {
	return func(c *Chrome) { c.breakpadTestMode = false }
}

// ExtraArgs returns an option that can be passed to New to append additional arguments to Chrome's command line.
func ExtraArgs(args ...string) option {
	return func(c *Chrome) { c.extraArgs = append(c.extraArgs, args...) }
}

// UnpackedExtension returns an option that can be passed to New to make Chrome load an unpacked
// extension in the supplied directory.
// Ownership of the extension directory and its contents may be modified by New.
func UnpackedExtension(dir string) option {
	return func(c *Chrome) { c.extDirs = append(c.extDirs, dir) }
}

// Chrome interacts with the currently-running Chrome instance via the
// Chrome DevTools protocol (https://chromedevtools.github.io/devtools-protocol/).
type Chrome struct {
	devsess *cdputil.Session // DevTools session

	user, pass, gaiaID string // login credentials
	normalizedUser     string // user with domain added, periods removed, etc.
	keepState          bool
	loginMode          loginMode
	region             string
	policyMode         policyMode
	arcMode            arcMode
	restrictARCCPU     bool // a flag to control cpu restrictions on ARC
	// If breakpadTestMode is true, tell Chrome's breakpad to always write
	// dumps directly to a hardcoded directory.
	breakpadTestMode bool
	extraArgs        []string

	extDirs     []string // directories containing all unpacked extensions to load
	testExtID   string   // ID for test extension exposing APIs
	testExtDir  string   // dir containing test extension
	testExtConn *Conn    // connection to test extension exposing APIs

	watcher   *browserWatcher // tries to catch Chrome restarts
	logMaster *jslog.Master   // collects JS console output
}

// User returns the username that was used to log in to Chrome.
func (c *Chrome) User() string { return c.user }

// DebugAddrPort returns the addr:port at which Chrome is listening for DevTools connections,
// e.g. "127.0.0.1:38725". This port should not be accessed from outside of this package,
// but it is exposed so that the port's owner can be easily identified.
func (c *Chrome) DebugAddrPort() string {
	return c.devsess.DebugAddrPort()
}

// New restarts the ui job, tells Chrome to enable testing, and (by default) logs in.
// The NoLogin option can be passed to avoid logging in.
func New(ctx context.Context, opts ...option) (*Chrome, error) {
	if locked {
		panic("Cannot create Chrome instance while precondition is being used")
	}

	ctx, st := timing.Start(ctx, "chrome_new")
	defer st.End()

	c := &Chrome{
		user:             DefaultUser,
		pass:             DefaultPass,
		gaiaID:           defaultGaiaID,
		keepState:        false,
		loginMode:        fakeLogin,
		region:           "us",
		policyMode:       noPolicy,
		breakpadTestMode: true,
		watcher:          newBrowserWatcher(),
		logMaster:        jslog.NewMaster(),
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

	if err := checkSoftwareDeps(ctx); err != nil {
		return nil, err
	}

	// This works around https://crbug.com/358427.
	if c.loginMode == gaiaLogin {
		var err error
		if c.normalizedUser, err = session.NormalizeEmail(c.user); err != nil {
			return nil, errors.Wrapf(err, "failed to normalize email %q", c.user)
		}
	} else {
		c.normalizedUser = c.user
	}

	// Perform an early high-level check of cryptohomed to avoid
	// less-descriptive errors later if it's broken.
	if c.loginMode != noLogin {
		if err := cryptohome.CheckService(ctx); err != nil {
			// Log problems in cryptohomed's dependencies.
			for _, e := range cryptohome.CheckDeps(ctx) {
				testing.ContextLog(ctx, "Potential cryptohome issue: ", e)
			}
			return nil, err
		}
	}

	if err := c.prepareExtensions(ctx); err != nil {
		return nil, err
	}

	if err := c.restartChromeForTesting(ctx); err != nil {
		return nil, err
	}
	var err error
	if c.devsess, err = cdputil.NewSession(ctx); err != nil {
		return nil, c.chromeErr(err)
	}

	if c.loginMode != noLogin && !c.keepState {
		if err := cryptohome.RemoveUserDir(ctx, c.normalizedUser); err != nil {
			return nil, err
		}
	}

	switch c.loginMode {
	case fakeLogin, gaiaLogin:
		if err := c.logIn(ctx); err != nil {
			return nil, err
		}
	case guestLogin:
		if err := c.logInAsGuest(ctx); err != nil {
			return nil, err
		}
	}

	toClose = nil
	return c, nil
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

// Close disconnects from Chrome and cleans up standard extensions.
// To avoid delays between tests, the ui job (and by extension, Chrome) is not restarted,
// so the current user (if any) remains logged in.
func (c *Chrome) Close(ctx context.Context) error {
	if locked {
		panic("Do not call Close while precondition is being used")
	}

	if c.testExtConn != nil {
		c.testExtConn.locked = false
		c.testExtConn.Close()
	}
	if len(c.testExtDir) > 0 {
		os.RemoveAll(c.testExtDir)
	}

	if c.devsess != nil {
		c.devsess.Close(ctx)
	}

	if c.watcher != nil {
		c.watcher.close()
	}

	if dir, ok := testing.ContextOutDir(ctx); ok {
		c.logMaster.Save(filepath.Join(dir, "jslog.txt"))
	}
	c.logMaster.Close()

	return nil
}

// ResetState attempts to reset Chrome's state (e.g. by closing all pages).
// Tests typically do not need to call this; it is exposed primarily for other packages.
func (c *Chrome) ResetState(ctx context.Context) error {
	testing.ContextLog(ctx, "Resetting Chrome's state")
	ctx, st := timing.Start(ctx, "reset_chrome")
	defer st.End()

	// Try to close all "normal" pages and apps.
	targets, err := c.getDevtoolTargets(ctx, func(t *target.Info) bool {
		return t.Type == "page" || t.Type == "app"
	})
	if err != nil {
		return errors.Wrap(err, "failed to get targets")
	}
	if len(targets) > 0 {
		testing.ContextLogf(ctx, "Closing %d target(s)", len(targets))
		for _, t := range targets {
			args := &target.CloseTargetArgs{TargetID: t.TargetID}
			if reply, err := c.devsess.Client.Target.CloseTarget(ctx, args); err != nil {
				testing.ContextLogf(ctx, "Failed to close %v: %v", t.URL, err)
			} else if !reply.Success {
				testing.ContextLogf(ctx, "Failed to close %v: unknown failure", t.URL)
			}
		}
	}
	return nil
}

// chromeErr returns c.watcher.err() if non-nil or orig otherwise. This is useful for
// replacing "context deadline exceeded" errors that can occur when Chrome is crashing
// with more-descriptive ones.
func (c *Chrome) chromeErr(orig error) error {
	if c.watcher == nil {
		return orig
	}
	werr := c.watcher.err()
	if werr == nil {
		return orig
	}
	return werr
}

// prepareExtensions prepares extensions to be loaded by Chrome.
func (c *Chrome) prepareExtensions(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "prepare_extensions")
	defer st.End()

	// Write the built-in test extension.
	var err error
	if c.testExtDir, err = ioutil.TempDir("", "tast_test_api_extension."); err != nil {
		return err
	}
	if c.testExtID, err = writeTestExtension(c.testExtDir); err != nil {
		return err
	}
	c.extDirs = append(c.extDirs, c.testExtDir)

	// Chrome hangs with a nonsensical "Extension error: Failed to load extension
	// from: . Manifest file is missing or unreadable." error if an extension directory
	// is owned by another user.
	for _, dir := range c.extDirs {
		manifest := filepath.Join(dir, "manifest.json")
		if _, err = os.Stat(manifest); err != nil {
			return errors.Wrap(err, "missing extension manifest")
		}
		if err := chownContents(dir, chromeUser); err != nil {
			return err
		}
	}
	return nil

}

// restartChromeForTesting restarts the ui job, asks session_manager to enable Chrome testing,
// and waits for Chrome to listen on its debugging port.
func (c *Chrome) restartChromeForTesting(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "restart")
	defer st.End()

	if err := c.restartSession(ctx); err != nil {
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
		"--oobe-skip-postlogin",                      // Skip post-login screens.
		"--autoplay-policy=no-user-gesture-required", // Allow media autoplay.
		"--enable-experimental-extension-apis",       // Allow Chrome to use the Chrome Automation API.
		"--whitelisted-extension-id=" + c.testExtID,  // Whitelists the test extension to access all Chrome APIs.
		"--redirect-libassistant-logging",            // Redirect libassistant logging to /var/log/chrome/.
		"--no-startup-window",                        // Do not start up chrome://newtab by default to avoid unexpected patterns(doodle etc.)
		"--no-first-run",                             // Prevent showing up offer pages, e.g. google.com/chromebooks.
		"--cros-region=" + c.region,                  // Force the region.
		"--cros-regions-mode=hide",                   // Ignore default values in VPD.
	}

	if c.loginMode != gaiaLogin {
		args = append(args, "--disable-gaia-services")
	}
	if len(c.extDirs) > 0 {
		args = append(args, "--load-extension="+strings.Join(c.extDirs, ","))
	}
	switch c.policyMode {
	case noPolicy:
		args = append(args, "--profile-requires-policy=false")
	case fetchPolicy:
		args = append(args, "--profile-requires-policy=true")
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
		if !c.restrictARCCPU {
			args = append(args,
				// Disable CPU restrictions to let tests run faster
				"--disable-arc-cpu-restriction")
		}
	case arcSupported:
		// Allow ARC being enabled on the device to test ARC with real gaia accounts.
		args = append(args, "--arc-availability=officially-supported")
	}
	args = append(args, c.extraArgs...)
	var envVars []string
	if c.breakpadTestMode {
		envVars = append(envVars,
			"CHROME_HEADLESS=",
			"BREAKPAD_DUMP_LOCATION="+crash.ChromeCrashDir) // Write crash dumps outside cryptohome.
	}
	if _, err = sm.EnableChromeTesting(ctx, true, args, envVars); err != nil {
		return err
	}

	// The original browser process should be gone now, so start watching for the new one.
	c.watcher.start()
	return nil
}

// restartSession stops the "ui" job, clears policy files and the user's cryptohome if requested,
// and restarts the job.
func (c *Chrome) restartSession(ctx context.Context) error {
	testing.ContextLog(ctx, "Restarting ui job")
	ctx, st := timing.Start(ctx, "restart_ui")
	defer st.End()

	ctx, cancel := context.WithTimeout(ctx, uiRestartTimeout)
	defer cancel()

	if err := upstart.StopJob(ctx, "ui"); err != nil {
		return err
	}

	if !c.keepState {
		const chronosDir = "/home/chronos"
		// This always fails because /home/chronos is a mount point, but all files
		// under the directory should be removed.
		os.RemoveAll(chronosDir)
		if fis, err := ioutil.ReadDir(chronosDir); err != nil {
			return err
		} else if len(fis) > 0 {
			return errors.Errorf("failed to clear %s: failed to remove %q", chronosDir, fis[0].Name())
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
func (c *Chrome) NewConn(ctx context.Context, url string) (*Conn, error) {
	if url == "" {
		testing.ContextLog(ctx, "Creating new blank page")
	} else {
		testing.ContextLog(ctx, "Creating new page with URL ", url)
	}
	reply, err := c.devsess.Client.Target.CreateTarget(ctx, &target.CreateTargetArgs{URL: url})
	if err != nil {
		return nil, err
	}

	conn, err := c.newConnInternal(ctx, reply.TargetID, url)
	if err != nil {
		return nil, err
	}
	if url != "" && url != blankURL {
		if err := conn.WaitForExpr(ctx, fmt.Sprintf("location.href !== %q", blankURL)); err != nil {
			return nil, errors.Wrap(err, "failed to wait for navigation")
		}
	}
	if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		return nil, errors.Wrap(err, "failed to wait for loading")
	}
	return conn, nil
}

// newConnInternal is a convenience function that creates a new Conn connected to the specified target.
// url is only used for logging JavaScript console messages.
func (c *Chrome) newConnInternal(ctx context.Context, id target.ID, url string) (*Conn, error) {
	return newConn(ctx, c.devsess.Manager, id, c.logMaster, url, c.chromeErr)
}

// Target contains information about an available debugging target to which a connection can be established.
type Target struct {
	// URL contains the URL of the resource currently loaded by the target.
	URL string
	// The type of the target. It's obtained from target.Info.Type.
	Type string
}

func newTarget(t *target.Info) *Target {
	return &Target{URL: t.URL, Type: t.Type}
}

// TargetMatcher is a caller-provided function that matches targets with specific characteristics.
type TargetMatcher func(t *Target) bool

// MatchTargetURL returns a TargetMatcher that matches targets with the supplied URL.
func MatchTargetURL(url string) TargetMatcher {
	return func(t *Target) bool { return t.URL == url }
}

// NewConnForTarget iterates through all available targets and returns a connection to the
// first one that is matched by tm. It polls until the target is found or ctx's deadline expires.
// An error is returned if no target is found, tm matches multiple targets, or the connection cannot
// be established.
//
//	f := func(t *Target) bool { return t.URL == "http://example.net/" }
//	conn, err := cr.NewConnForTarget(ctx, f)
func (c *Chrome) NewConnForTarget(ctx context.Context, tm TargetMatcher) (*Conn, error) {
	var errNoMatch = errors.New("no targets matched")

	matchAll := func(t *target.Info) bool { return true }

	var all, matched []*target.Info
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		all, err = c.getDevtoolTargets(ctx, matchAll)
		if err != nil {
			return c.chromeErr(err)
		}
		matched = []*target.Info{}
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
		return nil, errors.Errorf("%d targets found", len(matched))
	}
	t := matched[0]
	return c.newConnInternal(ctx, t.TargetID, t.URL)
}

// ExtensionBackgroundPageURL returns the URL to the background page for
// the extension with the supplied ID.
func ExtensionBackgroundPageURL(extID string) string {
	return "chrome-extension://" + extID + "/_generated_background_page.html"
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

	bgURL := ExtensionBackgroundPageURL(c.testExtID)
	testing.ContextLog(ctx, "Waiting for test API extension at ", bgURL)
	var err error
	if c.testExtConn, err = c.NewConnForTarget(ctx, MatchTargetURL(bgURL)); err != nil {
		return nil, err
	}
	c.testExtConn.locked = true

	// Ensure that we don't attempt to use the extension before its APIs are available: https://crbug.com/789313
	if err := c.testExtConn.WaitForExpr(ctx, "chrome.autotestPrivate"); err != nil {
		return nil, errors.Wrap(err, "chrome.autotestPrivate unavailable")
	}

	testing.ContextLog(ctx, "Test API extension is ready")
	return c.testExtConn, nil
}

// getDevtoolTargets returns all DevTools targets matched by f.
func (c *Chrome) getDevtoolTargets(ctx context.Context, f func(*target.Info) bool) ([]*target.Info, error) {
	reply, err := c.devsess.Client.Target.GetTargets(ctx)
	if err != nil {
		return nil, err
	}

	var matches []*target.Info
	for i := range reply.TargetInfos {
		t := reply.TargetInfos[i]
		if f(&t) {
			matches = append(matches, &t)
		}
	}
	return matches, nil
}

// getFirstOOBETarget returns the first OOBE-related DevTools target that it finds.
// nil is returned if no target is found.
func (c *Chrome) getFirstOOBETarget(ctx context.Context) (*target.Info, error) {
	targets, err := c.getDevtoolTargets(ctx, func(t *target.Info) bool {
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

// waitForOOBEConnection waits for that the OOBE page is shown, then returns
// a connection to the page. The caller must close the returned connection.
func (c *Chrome) waitForOOBEConnection(ctx context.Context) (*Conn, error) {
	testing.ContextLog(ctx, "Finding OOBE DevTools target")
	ctx, st := timing.Start(ctx, "wait_for_oobe")
	defer st.End()

	var target *target.Info
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		if target, err = c.getFirstOOBETarget(ctx); err != nil {
			return err
		} else if target == nil {
			return errors.Errorf("no %s target", oobePrefix)
		}
		return nil
	}, loginPollOpts); err != nil {
		return nil, errors.Wrap(c.chromeErr(err), "OOBE target not found")
	}

	conn, err := c.newConnInternal(ctx, target.TargetID, target.URL)
	if err != nil {
		return nil, err
	}
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()

	// Cribbed from telemetry/internal/backends/chrome/cros_browser_backend.py in Catapult.
	testing.ContextLog(ctx, "Waiting for OOBE")
	if err = conn.WaitForExpr(ctx, "typeof Oobe == 'function' && Oobe.readyForTesting"); err != nil {
		return nil, errors.Wrap(c.chromeErr(err), "OOBE didn't show up (Oobe.readyForTesting not found)")
	}

	connToRet := conn
	conn = nil
	return connToRet, nil
}

// logIn logs in to a freshly-restarted Chrome instance.
// It waits for the login process to complete before returning.
func (c *Chrome) logIn(ctx context.Context) error {
	conn, err := c.waitForOOBEConnection(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	testing.ContextLogf(ctx, "Logging in as user %q", c.user)
	ctx, st := timing.Start(ctx, "login")
	defer st.End()

	switch c.loginMode {
	case fakeLogin:
		if err = conn.Exec(ctx, fmt.Sprintf("Oobe.loginForTesting('%s', '%s', '%s', false)", c.user, c.pass, c.gaiaID)); err != nil {
			return err
		}
	case gaiaLogin:
		if err = c.performGAIALogin(ctx, conn); err != nil {
			return err
		}
	}

	if err = cryptohome.WaitForUserMount(ctx, c.normalizedUser); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Waiting for OOBE to be dismissed")
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		if t, err := c.getFirstOOBETarget(ctx); err != nil {
			// This is likely Chrome crash. So there's no chance that
			// waiting for the dismiss succeeds later. Quit the polling now.
			return testing.PollBreak(err)
		} else if t != nil {
			return errors.Errorf("%s target still exists", oobePrefix)
		}
		return nil
	}, loginPollOpts); err != nil {
		return errors.Wrap(c.chromeErr(err), "OOBE not dismissed")
	}

	return nil
}

// performGAIALogin waits for and interacts with the GAIA webview to perform login.
// This function is heavily based on NavigateGaiaLogin() in Catapult's
// telemetry/telemetry/internal/backends/chrome/oobe.py.
func (c *Chrome) performGAIALogin(ctx context.Context, oobeConn *Conn) error {
	if err := oobeConn.Exec(ctx, "Oobe.skipToLoginForTesting()"); err != nil {
		return err
	}

	isGAIAWebview := func(t *target.Info) bool {
		return t.Type == "webview" && strings.HasPrefix(t.URL, "https://accounts.google.com/")
	}

	testing.ContextLog(ctx, "Waiting for GAIA webview")
	var target *target.Info
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if targets, err := c.getDevtoolTargets(ctx, isGAIAWebview); err != nil {
			return err
		} else if len(targets) != 1 {
			return errors.Errorf("got %d GAIA targets; want 1", len(targets))
		} else {
			target = targets[0]
			return nil
		}
	}, loginPollOpts); err != nil {
		return errors.Wrap(c.chromeErr(err), "GAIA webview not found")
	}

	gaiaConn, err := c.newConnInternal(ctx, target.TargetID, target.URL)
	if err != nil {
		return errors.Wrap(c.chromeErr(err), "failed to connect to GAIA webview")
	}
	defer gaiaConn.Close()

	testing.ContextLog(ctx, "Performing GAIA login")
	for _, entry := range []struct{ inputID, nextID, value string }{
		{"identifierId", "identifierNext", c.user},
		{"password", "passwordNext", c.pass},
	} {
		for _, id := range []string{entry.inputID, entry.nextID} {
			if err := gaiaConn.WaitForExpr(ctx, fmt.Sprintf("document.getElementById(%q)", id)); err != nil {
				return errors.Wrapf(err, "failed to wait for %q element", id)
			}
		}
		// In GAIA v2, the 'password' element wraps an unidentified <input> element.
		// See https://crbug.com/739998 for more information.
		script := fmt.Sprintf(
			`(function() {
			let field = document.getElementById(%q);
			if (field.tagName !== 'INPUT') {
			  field = field.getElementsByTagName('INPUT')[0];
			}
			field.value = %q;
			document.getElementById(%q).click();
			})()`, entry.inputID, entry.value, entry.nextID)
		if err := gaiaConn.Exec(ctx, script); err != nil {
			return errors.Wrapf(err, "failed to use %q element", entry.inputID)
		}
	}

	return nil
}

// logInAsGuest logs in to a freshly-restarted Chrome instance as a guest user.
// It waits for the login process to complete before returning.
func (c *Chrome) logInAsGuest(ctx context.Context) error {
	oobeConn, err := c.waitForOOBEConnection(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if oobeConn != nil {
			oobeConn.Close()
		}
	}()

	testing.ContextLog(ctx, "Logging in as a guest user")
	ctx, st := timing.Start(ctx, "login_guest")
	defer st.End()

	// guestLoginForTesting() relaunches the browser. In advance,
	// remove the file at cdputil.DebuggingPortPath, which should be
	// recreated after the port gets ready.
	os.Remove(cdputil.DebuggingPortPath)
	// And stop the browser crash watcher temporarily.
	c.watcher.close()
	c.watcher = nil

	if err = oobeConn.Exec(ctx, "Oobe.guestLoginForTesting()"); err != nil {
		return err
	}

	// We also close our WebSocket connection to the browser.
	oobeConn.Close()
	oobeConn = nil
	c.devsess.Close(ctx)
	c.devsess = nil

	if err = cryptohome.WaitForUserMount(ctx, c.user); err != nil {
		return err
	}

	// The original browser process should be gone now, so start watching for the new one.
	c.watcher = newBrowserWatcher()
	c.watcher.start()

	// Then, get the possibly-changed debugging port and establish a new WebSocket connection.
	if c.devsess, err = cdputil.NewSession(ctx); err != nil {
		return c.chromeErr(err)
	}

	return nil
}

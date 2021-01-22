// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chrome implements a library used for communication with Chrome.
package chrome

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace"

	"chromiumos/tast/caller"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/internal/config"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/chrome/internal/extension"
	"chromiumos/tast/local/chrome/internal/login"
	"chromiumos/tast/local/chrome/internal/setup"
	"chromiumos/tast/local/chrome/jslog"
	"chromiumos/tast/local/cryptohome"
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

	if err := setup.PreflightCheck(ctx, cfg); err != nil {
		return nil, err
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

	if err := setup.RestartChromeForTesting(ctx, cfg, exts); err != nil {
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

// Conn represents a connection to a web content view, e.g. a tab.
type Conn = driver.Conn

// JSObject is a reference to a JavaScript object.
// JSObjects must be released or they will stop the JavaScript GC from freeing the memory they reference.
type JSObject = driver.JSObject

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

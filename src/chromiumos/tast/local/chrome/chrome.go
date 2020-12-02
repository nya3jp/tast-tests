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
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace"
	"github.com/golang/protobuf/proto"
	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/caller"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/jslog"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/minidump"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/shill"
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

	// testExtensionKey is a manifest key of the autotest extension.
	testExtensionKey = "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDuUZGKCDbff6IRaxa4Pue7PPkxwPaNhGT3JEqppEsNWFjM80imEdqMbf3lrWqEfaHgaNku7nlpwPO1mu3/4Hr+XdNa5MhfnOnuPee4hyTLwOs3Vzz81wpbdzUxZSi2OmqMyI5oTaBYICfNHLwcuc65N5dbt6WKGeKgTpp4v7j7zwIDAQAB"

	// TestExtensionID is an extension ID of the autotest extension. It
	// corresponds to testExtensionKey.
	TestExtensionID = "behllobkkfkfnphdnhnkndlbkcpglgmj"

	// signinProfileTestExtensionID is an id of the test extension which is
	// allowed for signin profile (see http://crrev.com/772709 for details).
	// It corresponds to Var("ui.signinProfileTestExtensionManifestKey").
	signinProfileTestExtensionID = "mecfefiddjlmabpeilblgegnbioikfmp"
)

const (
	// DefaultUser contains the email address used to log into Chrome when authentication credentials are not supplied.
	DefaultUser = "testuser@gmail.com"

	// DefaultPass contains the password we use to log into the DefaultUser account.
	DefaultPass   = "testpass"
	defaultGaiaID = "gaia-id"

	oobePrefix = "chrome://oobe"

	// BlankURL is the URL corresponding to the about:blank page.
	BlankURL = "about:blank"

	// localPassword is used in OOBE login screen. When contact email approval flow is used,
	// there is no password supplied by the user and this local password will be used to encrypt
	// cryptohome instead.
	localPassword = "test0000"
)

// Virtual keyboard background page url.
const vkBackgroundPageURL = "chrome-extension://jkghodnilhceideoidjikpgommlajknk/background.html"

// Use a low polling interval while waiting for conditions during login, as this code is shared by many tests.
var loginPollOpts = &testing.PollOptions{Interval: 10 * time.Millisecond}

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
	"chromiumos/tast/local/wpr",
}

//  domainRe is a regex used to obtain the domain (without top level domain) out of an email string.
//  e.g. a@managedchrome.com -> [a@managedchrome.com managedchrome] and
//  ex2@domainp1.domainp2.com -> [ex2@domainp1.domainp2.com domainp1.domainp2]
var domainRe = regexp.MustCompile(`^[^@]+@([^@]+)\.[^.@]*$`)

//  fullDomainRe is a regex used to obtain the full domain (with top level domain) out of an email string.
//  e.g. a@managedchrome.com -> [a@managedchrome.com managedchrome.com] and
//  ex2@domainp1.domainp2.com -> [ex2@domainp1.domainp2.com domainp1.domainp2.com]
var fullDomainRe = regexp.MustCompile(`^[^@]+@([^@]+)$`)

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

// authType describes the type of authentication to be used in GAIA.
type authType string

const (
	unknownAuth  authType = ""         // cannot determine the authentication type
	passwordAuth authType = "password" // password based authentication
	contactAuth  authType = "contact"  // contact email approval based authentication
)

// Option is a self-referential function can be used to configure Chrome.
// See https://commandcenter.blogspot.com.au/2014/01/self-referential-functions-and-design.html
// for details about this pattern.
type Option func(c *Chrome)

// EnableWebAppInstall returns an Option that can be passed to enable web app auto-install after user login.
// By default web app auto-install is disabled to reduce network traffic in test environment.
// See https://crbug.com/1076660 for more details.
func EnableWebAppInstall() Option {
	return func(c *Chrome) { c.installWebApp = true }
}

// EnableLoginVerboseLogs returns an Option that enables verbose logging for some login-related files.
func EnableLoginVerboseLogs() Option {
	return func(c *Chrome) { c.enableLoginVerboseLogs = true }
}

// VKEnabled returns an Option that force enable virtual keyboard.
// VKEnabled option appends "--enable-virtual-keyboard" to chrome initialization and also checks VK connection after user login.
// Note: This option can not be used by ARC tests as some boards block VK background from presence.
func VKEnabled() Option {
	return func(c *Chrome) { c.vkEnabled = true }
}

// Auth returns an Option that can be passed to New to configure the login credentials used by Chrome.
// Please do not check in real credentials to public repositories when using this in conjunction with GAIALogin.
func Auth(user, pass, gaiaID string) Option {
	return func(c *Chrome) {
		c.user = user
		c.pass = pass
		c.gaiaID = gaiaID
	}
}

// Contact returns an Option that can be passed to New to configure the contact email used by Chrome for
// cross account challenge (go/ota-security). Please do not check in real credentials to public repositories
// when using this in conjunction with GAIALogin.
func Contact(contact string) Option {
	return func(c *Chrome) {
		c.contact = contact
	}
}

// ParentAuth returns an Option that can be passed to New to configure the login credentials of a parent user.
// If the GAIA account specified by Auth is a supervised child user, this credential is used to go through the unicorn login flow.
// Please do not check in real credentials to public repositories when using this in conjunction with GAIALogin.
func ParentAuth(parentUser, parentPass string) Option {
	return func(c *Chrome) {
		c.parentUser = parentUser
		c.parentPass = parentPass
	}
}

// KeepState returns an Option that can be passed to New to preserve the state such as
// files under /home/chronos and the user's existing cryptohome (if any) instead of
// wiping them before logging in.
func KeepState() Option {
	return func(c *Chrome) { c.keepState = true }
}

// DeferLogin returns an option that instructs chrome.New to return before logging into a session.
// After successful return of chrome.New, you can call ContinueLogin to continue login.
func DeferLogin() Option {
	return func(c *Chrome) { c.deferLogin = true }
}

// GAIALogin returns an Option that can be passed to New to perform a real GAIA-based login rather
// than the default fake login.
func GAIALogin() Option {
	return func(c *Chrome) { c.loginMode = gaiaLogin }
}

// NoLogin returns an Option that can be passed to New to avoid logging in.
// Chrome is still restarted with testing-friendly behavior.
func NoLogin() Option {
	return func(c *Chrome) { c.loginMode = noLogin }
}

// GuestLogin returns an Option that can be passed to New to log in as guest
// user.
func GuestLogin() Option {
	return func(c *Chrome) {
		c.loginMode = guestLogin
		c.user = cryptohome.GuestUser
	}
}

// DontSkipOOBEAfterLogin returns an Option that can be passed to stay in OOBE after user login.
func DontSkipOOBEAfterLogin() Option {
	return func(c *Chrome) {
		c.skipOOBEAfterLogin = false
	}
}

// Region returns an Option that can be passed to New to set the region deciding
// the locale used in the OOBE screen and the user sessions. region is a
// two-letter code such as "us", "fr", or "ja".
func Region(region string) Option {
	return func(c *Chrome) {
		c.region = region
	}
}

// ProdPolicy returns an option that can be passed to New to let the device do a
// policy fetch upon login. By default, policies are not fetched.
// The default Device Management service is used.
func ProdPolicy() Option {
	return func(c *Chrome) {
		c.policyEnabled = true
		c.dmsAddr = ""
	}
}

// DMSPolicy returns an option that can be passed to New to tell the device to fetch
// policies from the policy server at the given url. By default policies are not
// fetched.
func DMSPolicy(url string) Option {
	return func(c *Chrome) {
		c.policyEnabled = true
		c.dmsAddr = url
	}
}

// EnterpriseEnroll returns an Option that can be passed to New to enable Enterprise
// Enrollment
func EnterpriseEnroll() Option {
	return func(c *Chrome) { c.enroll = true }
}

// ARCDisabled returns an Option that can be passed to New to disable ARC.
func ARCDisabled() Option {
	return func(c *Chrome) { c.arcMode = arcDisabled }
}

// ARCEnabled returns an Option that can be passed to New to enable ARC (without Play Store)
// for the user session with mock GAIA account.
func ARCEnabled() Option {
	return func(c *Chrome) { c.arcMode = arcEnabled }
}

// ARCSupported returns an Option that can be passed to New to allow to enable ARC with Play Store gaia opt-in for the user
// session with real GAIA account.
// In this case ARC is not launched by default and is required to be launched by user policy or from UI.
func ARCSupported() Option {
	return func(c *Chrome) { c.arcMode = arcSupported }
}

// RestrictARCCPU returns an Option that can be passed to New which controls whether
// to let Chrome use CGroups to limit the CPU time of ARC when in the background.
// Most ARC-related tests should not pass this option.
func RestrictARCCPU() Option {
	return func(c *Chrome) { c.restrictARCCPU = true }
}

// CrashNormalMode tells the crash handling system to act like it would on a
// real device. If this option is not used, the Chrome instances created by this package
// will skip calling crash_reporter and write any dumps into /home/chronos/crash directly
// from breakpad. This option restores the normal behavior of calling crash_reporter.
func CrashNormalMode() Option {
	return func(c *Chrome) { c.breakpadTestMode = false }
}

// ExtraArgs returns an Option that can be passed to New to append additional arguments to Chrome's command line.
func ExtraArgs(args ...string) Option {
	return func(c *Chrome) { c.extraArgs = append(c.extraArgs, args...) }
}

// EnableFeatures returns an Option that can be passed to New to enable specific features in Chrome.
func EnableFeatures(features ...string) Option {
	return func(c *Chrome) { c.enableFeatures = append(c.enableFeatures, features...) }
}

// DisableFeatures returns an Option that can be passed to New to disable specific features in Chrome.
func DisableFeatures(features ...string) Option {
	return func(c *Chrome) { c.disableFeatures = append(c.disableFeatures, features...) }
}

// UnpackedExtension returns an Option that can be passed to New to make Chrome load an unpacked
// extension in the supplied directory.
// Ownership of the extension directory and its contents may be modified by New.
func UnpackedExtension(dir string) Option {
	return func(c *Chrome) { c.extDirs = append(c.extDirs, dir) }
}

// LoadSigninProfileExtension loads the test extension which is allowed to run in the signin profile context.
// Private manifest key should be passed (see ui.SigninProfileExtension for details).
func LoadSigninProfileExtension(key string) Option {
	return func(c *Chrome) { c.signinExtKey = key }
}

// Chrome interacts with the currently-running Chrome instance via the
// Chrome DevTools protocol (https://chromedevtools.github.io/devtools-protocol/).
type Chrome struct {
	devsess *cdputil.Session // DevTools session

	user, pass, gaiaID, contact string // login credentials
	normalizedUser              string // user with domain added, periods removed, etc.
	parentUser, parentPass      string // unicorn parent login credentials
	keepState                   bool
	deferLogin                  bool
	loginMode                   loginMode
	enableLoginVerboseLogs      bool // enable verbose logging in some login related files
	vkEnabled                   bool
	skipOOBEAfterLogin          bool // skip OOBE post user login
	installWebApp               bool // auto install essential apps after user login
	region                      string
	policyEnabled               bool   // flag to enable policy fetch
	dmsAddr                     string // Device Management URL, or empty if using default
	enroll                      bool   // whether device should be enrolled
	arcMode                     arcMode
	restrictARCCPU              bool // a flag to control cpu restrictions on ARC

	// If breakpadTestMode is true, tell Chrome's breakpad to always write
	// dumps directly to a hardcoded directory.
	breakpadTestMode bool
	extraArgs        []string
	enableFeatures   []string
	disableFeatures  []string

	extDirs     []string // directories containing all unpacked extensions to load
	testExtID   string   // ID for test extension exposing APIs
	testExtDir  string   // dir containing test extension
	testExtConn *Conn    // connection to test extension exposing APIs

	signinExtKey  string // private key for signin profile test extension manifest
	signinExtID   string // ID for signin profile test extension exposing APIs
	signinExtDir  string // dir containing signin test profile extension
	signinExtConn *Conn  // connection to signin profile test extension

	tracingStarted bool // true when tracing is started

	watcher       *browserWatcher   // tries to catch Chrome restarts
	logAggregator *jslog.Aggregator // collects JS console output
}

// User returns the username that was used to log in to Chrome.
func (c *Chrome) User() string { return c.user }

// TestExtID returns the ID of the extension that exposes test-only APIs.
func (c *Chrome) TestExtID() string { return c.testExtID }

// ExtDirs returns the directories holding the test extensions.
func (c *Chrome) ExtDirs() []string { return c.extDirs }

// DebugAddrPort returns the addr:port at which Chrome is listening for DevTools connections,
// e.g. "127.0.0.1:38725". This port should not be accessed from outside of this package,
// but it is exposed so that the port's owner can be easily identified.
func (c *Chrome) DebugAddrPort() string {
	return c.devsess.DebugAddrPort()
}

// New restarts the ui job, tells Chrome to enable testing, and (by default) logs in.
// The NoLogin option can be passed to avoid logging in.
func New(ctx context.Context, opts ...Option) (*Chrome, error) {
	if locked {
		panic("Cannot create Chrome instance while precondition is being used")
	}

	ctx, st := timing.Start(ctx, "chrome_new")
	defer st.End()

	c := &Chrome{
		user:                   DefaultUser,
		pass:                   DefaultPass,
		gaiaID:                 defaultGaiaID,
		keepState:              false,
		loginMode:              fakeLogin,
		vkEnabled:              false,
		skipOOBEAfterLogin:     true,
		enableLoginVerboseLogs: false,
		installWebApp:          false,
		region:                 "us",
		policyEnabled:          false,
		enroll:                 false,
		breakpadTestMode:       true,
		tracingStarted:         false,
		logAggregator:          jslog.NewAggregator(),
	}
	for _, opt := range opts {
		opt(c)
	}

	// TODO(rrsilva, crbug.com/1109176) - Disable login-related verbose logging
	// in all tests once the issue is solved.
	EnableLoginVerboseLogs()(c)

	// Cap the timeout to be certain length depending on the login mode. Sometimes
	// chrome.New may fail and get stuck on an unexpected screen. Without timeout,
	// it simply runs out the entire timeout. See https://crbug.com/1078873.
	timeout := LoginTimeout
	if c.loginMode == gaiaLogin {
		timeout = gaiaLoginTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

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

	if err := checkStateful(); err != nil {
		return nil, err
	}

	// This works around https://crbug.com/358427.
	if c.loginMode == gaiaLogin {
		var err error
		if c.normalizedUser, err = session.NormalizeEmail(c.user, true); err != nil {
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
			return nil, errors.Wrap(err, "failed to check cryptohome service")
		}
	}

	if err := c.PrepareExtensions(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to prepare extensions")
	}

	if err := c.restartChromeForTesting(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to restart chrome for testing")
	}
	var err error
	if c.devsess, err = cdputil.NewSession(ctx, cdputil.DebuggingPortPath, cdputil.WaitPort); err != nil {
		return nil, errors.Wrapf(c.chromeErr(err), "failed to establish connection to Chrome Debuggin Protocol with debugging port path=%q", cdputil.DebuggingPortPath)
	}

	if c.loginMode != noLogin && !c.keepState {
		if err := cryptohome.RemoveUserDir(ctx, c.normalizedUser); err != nil {
			return nil, errors.Wrapf(err, "failed to remove cryptohome user directory for %s", c.normalizedUser)
		}
	}

	if !c.deferLogin && (c.loginMode != noLogin) {
		if err := c.logIn(ctx); err != nil {
			return nil, err
		}
	}

	// VK uses different extension instance in login profile and user profile.
	// BackgroundConn will wait until the background connection is unique.
	if c.vkEnabled {
		// Background target from login persists for a few seconds, causing 2 background targets.
		// Polling until connected to the unique target.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			bconn, err := c.NewConnForTarget(ctx, MatchTargetURL(vkBackgroundPageURL))
			if err != nil {
				return err
			}
			bconn.Close()
			return nil
		}, &testing.PollOptions{Timeout: 60 * time.Second, Interval: 1 * time.Second}); err != nil {
			return nil, errors.Wrap(err, "failed to wait for unique virtual keyboard background target")
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

	var firstErr error
	if c.watcher != nil {
		firstErr = c.watcher.close()
	}

	if dir, ok := testing.ContextOutDir(ctx); ok {
		c.logAggregator.Save(filepath.Join(dir, "jslog.txt"))
	}
	c.logAggregator.Close()

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
	targetFilter := func(t *target.Info) bool {
		return t.Type == "page" || t.Type == "app"
	}
	targets, err := c.devsess.FindTargets(ctx, targetFilter)
	if err != nil {
		return errors.Wrap(err, "failed to get targets")
	}
	var closingTargets []*target.Info
	if len(targets) > 0 {
		testing.ContextLogf(ctx, "Closing %d target(s)", len(targets))
		for _, t := range targets {
			if err := c.devsess.CloseTarget(ctx, t.TargetID); err != nil {
				testing.ContextLogf(ctx, "Failed to close %v: %v", t.URL, err)
			} else {
				// Record all targets that have promised to close
				closingTargets = append(closingTargets, t)
			}
		}
	}
	// Wait for the targets to finish closing
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		targets, err := c.devsess.FindTargets(ctx, targetFilter)
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
	if c.tracingStarted {
		// As noted in the comment of c.StartTracing, the tracingStarted flag is
		// marked before actually StartTracing request is sent because
		// StartTracing's failure doesn't necessarily mean that tracing isn't
		// started. So at this point, c.StopTracing may fail if StartTracing failed
		// and tracing actually didn't start. Because of that, StopTracing's error
		// wouldn't cause an error of ResetState, but simply reporting the error
		// message.
		if _, err := c.StopTracing(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to stop tracing: ", err)
		}
	}

	// If testExtCon was created, free all remote JS objects in the TastObjectGroup.
	if c.testExtConn != nil {
		if err := c.testExtConn.co.ReleaseObjectGroup(ctx, cdputil.TastObjectGroup); err != nil {
			return errors.Wrap(err, "failed to free tast remote JS object group")
		}
	}

	tconn, err := c.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get test API connection")
	}

	if c.vkEnabled {
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
	if err := tconn.Eval(ctx, "tast.promisify(chrome.autotestPrivate.disableAutomation)()", nil); err != nil {
		return errors.Wrap(err, "failed to disable the automation feature")
	}

	// Reloading the test extension contents to clear all of Javascript objects.
	// This also resets the internal state of automation tree, so without
	// reloading, disableAutomation above would cause failures.
	testing.ContextLog(ctx, "Reloading the extension process")
	if err := tconn.Eval(ctx, "location.reload()", nil); err != nil {
		return errors.Wrap(err, "failed to reload the test extension")
	}
	if err := tconn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		return errors.Wrap(err, "failed to wait for the ready state")
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

// PrepareExtensions prepares extensions to be loaded by Chrome.
func (c *Chrome) PrepareExtensions(ctx context.Context) error {
	ctx, st := timing.Start(ctx, "prepare_extensions")
	defer st.End()

	// Write the built-in test extension.
	var err error
	if c.testExtDir, err = ioutil.TempDir("", "tast_test_api_extension."); err != nil {
		return err
	}
	if c.testExtID, err = writeTestExtension(c.testExtDir, testExtensionKey); err != nil {
		return err
	}
	if c.testExtID != TestExtensionID {
		return errors.Errorf("unexpected extension ID: got %q; want %q", c.testExtID, TestExtensionID)
	}
	c.extDirs = append(c.extDirs, c.testExtDir)

	// Chrome hangs with a nonsensical "Extension error: Failed to load extension
	// from: . Manifest file is missing or unreadable." error if an extension directory
	// is owned by another user.
	dirsToChown := c.extDirs

	// Write the signin profile test extension.
	if len(c.signinExtKey) > 0 {
		var err error
		if c.signinExtDir, err = ioutil.TempDir("", "tast_test_signin_api_extension."); err != nil {
			return err
		}
		if c.signinExtID, err = writeTestExtension(c.signinExtDir, c.signinExtKey); err != nil {
			return err
		}
		if c.signinExtID != signinProfileTestExtensionID {
			return errors.Errorf("unexpected extension ID: got %q; want %q", c.signinExtID, signinProfileTestExtensionID)
		}
		dirsToChown = append(dirsToChown, c.signinExtDir)
	}

	for _, dir := range dirsToChown {
		manifest := filepath.Join(dir, "manifest.json")
		if _, err = os.Stat(manifest); err != nil {
			return errors.Wrap(err, "missing extension manifest")
		}
		if err := ChownContentsToChrome(dir); err != nil {
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
		"--autoplay-policy=no-user-gesture-required", // Allow media autoplay.
		"--enable-experimental-extension-apis",       // Allow Chrome to use the Chrome Automation API.
		"--redirect-libassistant-logging",            // Redirect libassistant logging to /var/log/chrome/.
		"--no-startup-window",                        // Do not start up chrome://newtab by default to avoid unexpected patterns(doodle etc.)
		"--no-first-run",                             // Prevent showing up offer pages, e.g. google.com/chromebooks.
		"--cros-region=" + c.region,                  // Force the region.
		"--cros-regions-mode=hide",                   // Ignore default values in VPD.
		"--enable-oobe-test-api",                     // Enable OOBE helper functions for authentication.
	}
	if c.enroll {
		args = append(args, "--disable-policy-key-verification") // Remove policy key verification for fake enrollment
	}

	if c.skipOOBEAfterLogin {
		args = append(args, "--oobe-skip-postlogin")
	}

	if !c.installWebApp {
		args = append(args, "--disable-features=DefaultWebAppInstallation")
	}

	if c.vkEnabled {
		args = append(args, "--enable-virtual-keyboard")
	}

	// Enable verbose logging on some enrollment related files.
	if c.enableLoginVerboseLogs {
		args = append(args,
			"--vmodule="+strings.Join([]string{
				"*auto_enrollment_check_screen*=1",
				"*enrollment_screen*=1",
				"*login_display_host_common*=1",
				"*wizard_controller*=1",
				"*auto_enrollment_controller*=1"}, ","))
	}

	if c.loginMode != gaiaLogin {
		args = append(args, "--disable-gaia-services")
	}
	if len(c.extDirs) > 0 {
		args = append(args, "--load-extension="+strings.Join(c.extDirs, ","))
	}
	if len(c.signinExtDir) > 0 {
		args = append(args, "--load-signin-profile-test-extension="+c.signinExtDir)
		args = append(args, "--whitelisted-extension-id="+c.signinExtID) // Allowlists the signin profile's test extension to access all Chrome APIs.
	} else {
		args = append(args, "--whitelisted-extension-id="+c.testExtID) // Allowlists the test extension to access all Chrome APIs.
	}
	if c.policyEnabled {
		args = append(args, "--profile-requires-policy=true")
	} else {
		args = append(args, "--profile-requires-policy=false")
	}
	if c.dmsAddr != "" {
		args = append(args, "--device-management-url="+c.dmsAddr)
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
	case arcSupported:
		// Allow ARC being enabled on the device to test ARC with real gaia accounts.
		args = append(args, "--arc-availability=officially-supported")
	}
	if c.arcMode == arcEnabled || c.arcMode == arcSupported {
		args = append(args,
			// Do not sync the locale with ARC.
			"--arc-disable-locale-sync",
			// Do not update Play Store automatically.
			"--arc-play-store-auto-update=off",
			// Make 1 Android pixel always match 1 Chrome devicePixel.
			"--force-remote-shell-scale=")
		if !c.restrictARCCPU {
			args = append(args,
				// Disable CPU restrictions to let tests run faster
				"--disable-arc-cpu-restriction")
		}
	}

	if len(c.enableFeatures) != 0 {
		args = append(args, "--enable-features="+strings.Join(c.enableFeatures, ","))
	}

	if len(c.disableFeatures) != 0 {
		args = append(args, "--disable-features="+strings.Join(c.disableFeatures, ","))
	}

	args = append(args, c.extraArgs...)
	var envVars []string
	if c.breakpadTestMode {
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

	// Start watching the new browser.
	c.watcher = newBrowserWatcher()
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
	if url == "" {
		testing.ContextLog(ctx, "Creating new blank page")
	} else {
		testing.ContextLog(ctx, "Creating new page with URL ", url)
	}
	targetID, err := c.devsess.CreateTarget(ctx, url, opts...)
	if err != nil {
		return nil, err
	}

	conn, err := c.newConnInternal(ctx, targetID, url)
	if err != nil {
		return nil, err
	}
	if url != "" && url != BlankURL {
		if err := conn.WaitForExpr(ctx, fmt.Sprintf("location.href !== %q", BlankURL)); err != nil {
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
	return NewConn(ctx, c.devsess, id, c.logAggregator, url, c.chromeErr)
}

// TargetMatcher is a caller-provided function that matches targets with specific characteristics.
type TargetMatcher = cdputil.TargetMatcher

// MatchTargetURL returns a TargetMatcher that matches targets with the supplied URL.
func MatchTargetURL(url string) TargetMatcher {
	return func(t *target.Info) bool { return t.URL == url }
}

// NewConnForTarget iterates through all available targets and returns a connection to the
// first one that is matched by tm. It polls until the target is found or ctx's deadline expires.
// An error is returned if no target is found, tm matches multiple targets, or the connection cannot
// be established.
//
//	f := func(t *Target) bool { return t.URL == "http://example.net/" }
//	conn, err := cr.NewConnForTarget(ctx, f)
func (c *Chrome) NewConnForTarget(ctx context.Context, tm TargetMatcher) (*Conn, error) {
	t, err := c.devsess.WaitForTarget(ctx, tm)
	if err != nil {
		return nil, c.chromeErr(err)
	}
	return c.newConnInternal(ctx, t.TargetID, t.URL)
}

// ExtensionBackgroundPageURL returns the URL to the background page for
// the extension with the supplied ID.
func ExtensionBackgroundPageURL(extID string) string {
	return "chrome-extension://" + extID + "/_generated_background_page.html"
}

// TestConn is a connection to the Tast test extension's background page.
// cf) crbug.com/1043590
type TestConn struct {
	*Conn
}

// TestAPIConn returns a shared connection to the test API extension's
// background page (which can be used to access various APIs). The connection is
// lazily created, and this function will block until the extension is loaded or
// ctx's deadline is reached. The caller should not close the returned
// connection; it will be closed automatically by Close.
func (c *Chrome) TestAPIConn(ctx context.Context) (*TestConn, error) {
	return c.testAPIConnFor(ctx, &c.testExtConn, c.testExtID)
}

// SigninProfileTestAPIConn is the same as TestAPIConn, but for the signin
// profile test extension.
func (c *Chrome) SigninProfileTestAPIConn(ctx context.Context) (*TestConn, error) {
	return c.testAPIConnFor(ctx, &c.signinExtConn, c.signinExtID)
}

// testAPIConnFor builds a test API connection to the extension specified by
// extID.
func (c *Chrome) testAPIConnFor(ctx context.Context, extConn **Conn, extID string) (*TestConn, error) {
	if *extConn != nil {
		return &TestConn{*extConn}, nil
	}

	bgURL := ExtensionBackgroundPageURL(extID)
	testing.ContextLog(ctx, "Waiting for test API extension at ", bgURL)
	var err error
	if *extConn, err = c.NewConnForTarget(ctx, MatchTargetURL(bgURL)); err != nil {
		return nil, err
	}
	(*extConn).locked = true

	// Ensure that we don't attempt to use the extension before its APIs are available: https://crbug.com/789313
	if err := (*extConn).WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		return nil, errors.Wrap(err, "test API extension is unavailable")
	}

	// Wait for tast API to be available.
	if err := (*extConn).WaitForExpr(ctx, `typeof tast != 'undefined'`); err != nil {
		return nil, errors.Wrap(err, "tast API is unavailable")
	}

	if err := (*extConn).Exec(ctx, "chrome.autotestPrivate.initializeEvents()"); err != nil {
		return nil, errors.Wrap(err, "failed to initialize test API events")
	}

	testing.ContextLog(ctx, "Test API extension is ready")
	return &TestConn{*extConn}, nil
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

// getFirstOOBETarget returns the first OOBE-related DevTools target that it finds.
// nil is returned if no target is found.
func (c *Chrome) getFirstOOBETarget(ctx context.Context) (*target.Info, error) {
	targets, err := c.devsess.FindTargets(ctx, func(t *target.Info) bool {
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

// enterpriseEnrollTargets returns the Gaia WebView targets, which are used
// to help enrollment on the device.
// Returns nil if none are found.
func (c *Chrome) enterpriseEnrollTargets(ctx context.Context, userDomain string) ([]*target.Info, error) {
	isGAIAWebView := func(t *target.Info) bool {
		return t.Type == "webview" && strings.HasPrefix(t.URL, "https://accounts.google.com/")
	}

	targets, err := c.devsess.FindTargets(ctx, isGAIAWebView)
	if err != nil {
		return nil, err
	}

	// It's common for multiple targets to be returned.
	// We want to run the command specifically on the "apps" target.
	var enterpriseTargets []*target.Info
	for _, target := range targets {
		u, err := url.Parse(target.URL)
		if err != nil {
			continue
		}

		q := u.Query()
		clientID := q.Get("client_id")
		managedDomain := q.Get("manageddomain")

		if clientID != "" && managedDomain != "" {
			if strings.Contains(clientID, "apps.googleusercontent.com") &&
				strings.Contains(managedDomain, userDomain) {
				enterpriseTargets = append(enterpriseTargets, target)
			}
		}
	}

	return enterpriseTargets, nil
}

// WaitForOOBEConnection waits for that the OOBE page is shown, then returns
// a connection to the page. The caller must close the returned connection.
func (c *Chrome) WaitForOOBEConnection(ctx context.Context) (*Conn, error) {
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
	if err = conn.WaitForExpr(ctx, "typeof OobeAPI == 'object'"); err != nil {
		return nil, errors.Wrap(c.chromeErr(err), "OOBE didn't show up (OobeAPI not found)")
	}

	connToRet := conn
	conn = nil
	return connToRet, nil
}

// userDomain will return the "domain" section (without top level domain) of the c.user.
// e.g. something@managedchrome.com will return "managedchrome"
// or x@domainp1.domainp2.com would return "domainp1domainp2"
func (c *Chrome) userDomain() (string, error) {
	m := domainRe.FindStringSubmatch(c.user)
	// This check mandates the same format as the fake DM server.
	if len(m) != 2 {
		return "", errors.New("'user' must have exactly 1 '@' and atleast one '.' after the @")
	}
	return strings.Replace(m[1], ".", "", -1), nil
}

// fullUserDomain will return the full "domain" (including top level domain) of the c.user.
// e.g. something@managedchrome.com will return "managedchrome.com"
// or x@domainp1.domainp2.com would return "domainp1.domainp2.com"
func (c *Chrome) fullUserDomain() (string, error) {
	m := fullDomainRe.FindStringSubmatch(c.user)
	// If nothing is returned, the enrollment will fail.
	if len(m) != 2 {
		return "", errors.New("'user' must have exactly 1 '@'")
	}
	return m[1], nil
}

// waitForEnrollmentLoginScreen will wait for the Enrollment screen to complete
// and the Enrollment login screen to appear. If the login screen does not appear
// the testing.Poll will timeout.
func (c *Chrome) waitForEnrollmentLoginScreen(ctx context.Context) error {
	testing.ContextLog(ctx, "Waiting for enrollment to complete")
	fullDomain, err := c.fullUserDomain()
	if err != nil {
		return errors.Wrap(err, "no valid full user domain found")
	}
	loginBanner := fmt.Sprintf(`document.querySelectorAll('span[title=%q]').length;`,
		fullDomain)

	userDomain, err := c.userDomain()
	if err != nil {
		return errors.Wrap(err, "no vaid user domain found")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		gaiaTargets, err := c.enterpriseEnrollTargets(ctx, userDomain)
		if err != nil {
			return errors.Wrap(err, "no Enrollment webview targets")
		}
		for _, gaiaTarget := range gaiaTargets {
			webViewConn, err := c.NewConnForTarget(ctx, MatchTargetURL(gaiaTarget.URL))
			if err != nil {
				// If an error occurs during connection, continue to try.
				// Enrollment will only exceed if the eval below succeeds.
				continue
			}
			defer webViewConn.Close()
			content := -1
			if err := webViewConn.Eval(ctx, loginBanner, &content); err != nil {
				return err
			}
			// Found the login screen.
			if content == 1 {
				return nil
			}
		}
		return errors.New("Enterprise Enrollment login screen not found")
	}, &testing.PollOptions{Timeout: 45 * time.Second}); err != nil {
		return err
	}

	return nil
}

// enterpriseOOBELogin will complete the oobe login after Enrollment completes.
func (c *Chrome) enterpriseOOBELogin(ctx context.Context) error {
	if err := c.waitForEnrollmentLoginScreen(ctx); err != nil {
		return errors.Wrap(c.chromeErr(err), "could not enroll")
	}

	// Reconnect to OOBE to log in after enrollment.
	// See crrev.com/c/2144279 for details.
	conn, err := c.WaitForOOBEConnection(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to reconnect to OOBE after enrollment")
	}
	defer conn.Close()

	testing.ContextLog(ctx, "Performing login after enrollment")
	// Now login like "normal".
	if err := conn.Exec(ctx, fmt.Sprintf("Oobe.loginForTesting('%s', '%s', '%s', false)",
		c.user, c.pass, c.gaiaID)); err != nil {
		return err
	}

	return nil
}

// ContinueLogin continues login deferred by DeferLogin option. It is an error to call
// this method when DeferLogin option was not passed to New.
func (c *Chrome) ContinueLogin(ctx context.Context) error {
	if !c.deferLogin {
		return errors.New("ContinueLogin can be called once after DeferLogin option is used")
	}
	c.deferLogin = false
	if err := c.logIn(ctx); err != nil {
		return err
	}

	return nil
}

// logIn performs a user or guest login based on the loginMode.
func (c *Chrome) logIn(ctx context.Context) error {
	switch c.loginMode {
	case fakeLogin, gaiaLogin:
		if err := c.loginUser(ctx); err != nil {
			return err
		}
		// Clear all notifications after logging in so none will be shown at the beginning of tests.
		// TODO(crbug/1079235): move this outside of the switch once the test API is available in guest mode.
		tconn, err := c.TestAPIConn(ctx)
		if err != nil {
			return err
		}
		if err := tconn.Eval(ctx, "tast.promisify(chrome.autotestPrivate.removeAllNotifications)()", nil); err != nil {
			return errors.Wrap(err, "failed to clear notifications")
		}
	case guestLogin:
		if err := c.logInAsGuest(ctx); err != nil {
			return err
		}
	}
	return nil
}

// loginUser logs in to a freshly-restarted Chrome instance.
// It waits for the login process to complete before returning.
func (c *Chrome) loginUser(ctx context.Context) error {
	conn, err := c.WaitForOOBEConnection(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	testing.ContextLogf(ctx, "Logging in as user %q", c.user)
	ctx, st := timing.Start(ctx, "login")
	defer st.End()

	switch c.loginMode {
	case fakeLogin:
		if err = conn.Exec(ctx, fmt.Sprintf("Oobe.loginForTesting('%s', '%s', '%s', %t)",
			c.user, c.pass, c.gaiaID, c.enroll)); err != nil {
			return err
		}
	case gaiaLogin:
		// GAIA login requires Internet connectivity.
		if err := shill.WaitForOnline(ctx); err != nil {
			return err
		}
		if err = c.performGAIALogin(ctx, conn); err != nil {
			return err
		}
	}

	if c.enroll {
		if err := c.enterpriseOOBELogin(ctx); err != nil {
			return err
		}
	}

	if err = cryptohome.WaitForUserMount(ctx, c.normalizedUser); err != nil {
		return err
	}

	if c.skipOOBEAfterLogin {
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

	var url string
	if err := oobeConn.Eval(ctx, "window.location.href", &url); err != nil {
		return err
	}
	if strings.HasPrefix(url, "chrome://oobe/gaia-signin") {
		// Force show GAIA webview even if the cryptohome exists. When there is an existing
		// user on the device, the login screen would be chrome://oobe/gaia-signin instead
		// of the accounts.google.com webview. Use Oobe.showAddUserForTesting() to open that
		// webview so we can reuse the same login logic below.
		testing.ContextLogf(ctx, "Found %s, force opening GAIA webview", url)
		if err := oobeConn.Exec(ctx, "Oobe.showAddUserForTesting()"); err != nil {
			return err
		}
	}

	isGAIAWebView := func(t *target.Info) bool {
		return t.Type == "webview" && strings.HasPrefix(t.URL, "https://accounts.google.com/")
	}

	testing.ContextLog(ctx, "Waiting for GAIA webview")
	var target *target.Info
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if targets, err := c.devsess.FindTargets(ctx, isGAIAWebView); err != nil {
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

	// Fill in username.
	if err := insertGAIAField(ctx, gaiaConn, "#identifierId", c.user); err != nil {
		return errors.Wrap(err, "failed to fill username field")
	}
	if err := oobeConn.Exec(ctx, "Oobe.clickGaiaPrimaryButtonForTesting()"); err != nil {
		return errors.Wrap(err, "failed to click on the primary action button")
	}

	// Fill in password / contact email.
	authType, err := getAuthType(ctx, gaiaConn)
	if err != nil {
		return errors.Wrap(err, "could not determine the authentication type for this account")
	}
	if authType == passwordAuth {
		testing.ContextLog(ctx, "This account uses password authentication")
		if c.pass == "" {
			return errors.New("please supply a password with chrome.Auth()")
		}
		if err := insertGAIAField(ctx, gaiaConn, "input[name=password]", c.pass); err != nil {
			return errors.Wrap(err, "failed to fill in password field")
		}
	} else if authType == contactAuth {
		testing.ContextLog(ctx, "This account uses contact email authentication")
		if c.contact == "" {
			return errors.New("please supply a contact email with chrome.Contact()")
		}
		if err := insertGAIAField(ctx, gaiaConn, "input[name=email]", c.contact); err != nil {
			return errors.Wrap(err, "failed to fill in contact email field")
		}
	} else {
		return errors.Errorf("got an invalid authentication type (%q) for this account", authType)
	}
	if err := oobeConn.Exec(ctx, "Oobe.clickGaiaPrimaryButtonForTesting()"); err != nil {
		return errors.Wrap(err, "failed to click on the primary action button")
	}

	// Wait for contact email approval and fill in local password.
	if authType == contactAuth {
		testing.ContextLog(ctx, "Please go to https://g.co/verifyaccount to approve the login request")
		testing.ContextLog(ctx, "Waiting for approval")
		if err := oobeConn.WaitForExpr(ctx, "OobeAPI.screens.ConfirmSamlPasswordScreen.isVisible()"); err != nil {
			return errors.Wrap(err, "failed to wait for OOBE password screen")
		}
		testing.ContextLog(ctx, "The login request is approved. Entering local password")
		if err := oobeConn.Call(ctx, nil, `(pw) => { OobeAPI.screens.ConfirmSamlPasswordScreen.enterManualPasswords(pw); }`, localPassword); err != nil {
			return errors.Wrap(err, "failed to fill in local password field")
		}
	}

	// Perform Unicorn login if parent user given.
	if c.parentUser != "" {
		if err := c.performUnicornParentLogin(ctx, oobeConn, gaiaConn); err != nil {
			return err
		}
	}

	return nil
}

// getAuthType determines the authentication type by checking whether the current page
// is expecting a password or contact email input.
func getAuthType(ctx context.Context, gaiaConn *Conn) (authType, error) {
	const query = `
	(function() {
		if (document.getElementById('password')) {
			return 'password';
		}
		if (document.getElementsByName('email').length > 0) {
			return 'contact';
		}
		return "";
	})();
	`
	t := unknownAuth
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := gaiaConn.Eval(ctx, query, &t); err != nil {
			return err
		}
		if t == passwordAuth || t == contactAuth {
			return nil
		}
		return errors.New("failed to locate password or contact input field")
	}, loginPollOpts); err != nil {
		return unknownAuth, err
	}

	return t, nil
}

// insertGAIAField fills a field of the GAIA login form.
func insertGAIAField(ctx context.Context, gaiaConn *Conn, selector, value string) error {
	// Ensure that the input exists.
	if err := gaiaConn.WaitForExpr(ctx, fmt.Sprintf(
		"document.querySelector(%q)", selector)); err != nil {
		return errors.Wrapf(err, "failed to wait for %q element", selector)
	}
	// Ensure the input field is empty.
	// This confirms that we are not using the field before it is cleared.
	fieldReady := fmt.Sprintf(`
		(function() {
			const field = document.querySelector(%q);
			return field.value === "";
		})()`, selector)
	if err := gaiaConn.WaitForExpr(ctx, fieldReady); err != nil {
		return errors.Wrapf(err, "failed to wait for %q element to be empty", selector)
	}

	// Fill the field with value.
	script := fmt.Sprintf(`
		(function() {
			const field = document.querySelector(%q);
			field.value = %q;
		})()`, selector, value)
	if err := gaiaConn.Exec(ctx, script); err != nil {
		return errors.Wrapf(err, "failed to use %q element", selector)
	}
	return nil
}

// performUnicornParentLogin Logs in a parent account and accepts Unicorn permissions.
// This function is heavily based on NavigateUnicornLogin() in Catapult's
// telemetry/telemetry/internal/backends/chrome/oobe.py.
func (c *Chrome) performUnicornParentLogin(ctx context.Context, oobeConn, gaiaConn *Conn) error {
	normalizedParentUser, err := session.NormalizeEmail(c.parentUser, false)
	if err != nil {
		return errors.Wrapf(err, "failed to normalize email %q", c.user)
	}

	testing.ContextLogf(ctx, "Clicking button that matches parent email: %q", normalizedParentUser)
	buttonTextQuery := `
		(function() {
			const buttons = document.querySelectorAll('%[1]s');
			if (buttons === null){
				throw new Error('no buttons found on screen');
			}
			return [...buttons].map(button=>button.textContent);
		})();`

	clickButtonQuery := `
                (function() {
                        const buttons = document.querySelectorAll('%[1]s');
                        if (buttons === null){
                                throw new Error('no buttons found on screen');
                        }
                        for (const button of buttons) {
                                if (button.textContent.indexOf(%[2]q) !== -1) {
                                        button.click();
                                        return;
                                }
                        }
                        throw new Error(%[2]q + ' button not found');
                })();`
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var buttons []string
		if err := gaiaConn.Eval(ctx, fmt.Sprintf(buttonTextQuery, "[data-email]"), &buttons); err != nil {
			return err
		}
	NextButton:
		for _, button := range buttons {
			if len(button) < len(normalizedParentUser) {
				continue NextButton
			}
			// The end of button text contains the email.
			// Trim email to be the same length as normalizedParentUser.
			potentialEmail := button[len(button)-len(normalizedParentUser):]

			// Compare email to parent.
			for i := range normalizedParentUser {
				// Ignore wildcards.
				if potentialEmail[i] == '*' {
					continue
				}
				if potentialEmail[i] != normalizedParentUser[i] {
					continue NextButton
				}
			}

			// Button matches. Click it.
			return gaiaConn.Exec(ctx, fmt.Sprintf(clickButtonQuery, "[data-email]", button))
		}
		return errors.New("no button matches email")
	}, loginPollOpts); err != nil {
		return errors.Wrap(c.chromeErr(err), "failed to select parent user")
	}

	testing.ContextLog(ctx, "Typing parent password")
	if err := insertGAIAField(ctx, gaiaConn, "input[name=password]", c.parentPass); err != nil {
		return err
	}
	if err := oobeConn.Exec(ctx, "Oobe.clickGaiaPrimaryButtonForTesting()"); err != nil {
		return errors.Wrap(err, "failed to click on the primary action button")
	}

	testing.ContextLog(ctx, "Accepting Unicorn permissions")
	clickAgreeQuery := fmt.Sprintf(clickButtonQuery, "button", "agree")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return gaiaConn.Exec(ctx, clickAgreeQuery)
	}, loginPollOpts); err != nil {
		return errors.Wrap(c.chromeErr(err), "failed to accept Unicorn permissions")
	}

	return nil
}

// logInAsGuest logs in to a freshly-restarted Chrome instance as a guest user.
// It waits for the login process to complete before returning.
func (c *Chrome) logInAsGuest(ctx context.Context) error {
	oobeConn, err := c.WaitForOOBEConnection(ctx)
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
	err = c.watcher.close()
	c.watcher = nil // clear watcher anyway to avoid double close
	if err != nil {
		return err
	}

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

	// Then, get the possibly-changed debugging port and establish a new WebSocket connection.
	if c.devsess, err = cdputil.NewSession(ctx, cdputil.DebuggingPortPath, cdputil.WaitPort); err != nil {
		return c.chromeErr(err)
	}

	return nil
}

// IsTargetAvailable checks if there is any matched target.
func (c *Chrome) IsTargetAvailable(ctx context.Context, tm TargetMatcher) (bool, error) {
	targets, err := c.devsess.FindTargets(ctx, tm)
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
	// Note: even when StartTracing fails, it might be due to the case that the
	// StartTracing request is successfully sent to the browser and tracing
	// collection has started, but the context deadline is exceeded before Tast
	// receives the reply.  Therefore, tracingStarted flag is marked beforehand.
	c.tracingStarted = true
	return c.devsess.StartTracing(ctx, categories)
}

// StopTracing stops trace collection and returns the collected trace events.
func (c *Chrome) StopTracing(ctx context.Context) (*trace.Trace, error) {
	traces, err := c.devsess.StopTracing(ctx)
	if err != nil {
		return nil, err
	}
	c.tracingStarted = false
	return traces, nil
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

// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// resetTimeout is the timeout duration to trying reset of the current precondition.
const resetTimeout = 30 * time.Second

// PreData holds information made available to tests that specify preconditions.
type PreData struct {
	// Chrome is a connection to an already-started Chrome instance.
	// It cannot be closed by tests.
	Chrome *chrome.Chrome
	// ARC enables interaction with an already-started ARC environment.
	// It cannot be closed by tests.
	ARC *ARC
}

// Booted returns a precondition that ARC Container has already booted when a test is run.
//
// When adding a test, the testing.Test.Pre field may be set to the value returned by this function.
// Later, in the main test function, the value returned by testing.State.PreValue may be converted
// to a PreData containing already-initialized chrome.Chrome and ARC objects:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.PreValue().(arc.PreData)
//		conn, err := d.Chrome.NewConn(ctx, "http://www.example.org/")
//		...
//		cmd := d.ARC.Command(ctx, "dumpsys", "window", "displays")
//		...
//	}
//
// When using this precondition, tests cannot call New or chrome.New.
// The Chrome and ARC instances are also shared and cannot be closed by tests.
func Booted() testing.Precondition { return bootedPre }

// bootedPre is returned by Booted.
var bootedPre = &preImpl{
	name:    "arc_booted",
	timeout: resetTimeout + chrome.LoginTimeout + BootTimeout,
}

// VMBooted returns a precondition similar to Booted(). The only difference from Booted() is
// that ARC VM, and not the ARC Container, is enabled in this precondition.
func VMBooted() testing.Precondition { return vmBootedPre }

// vmBootedPre is returned by VMBooted.
var vmBootedPre = &preImpl{
	name:      "arcvm_booted",
	timeout:   resetTimeout + chrome.LoginTimeout + BootTimeout,
	extraArgs: []string{"--enable-arcvm"},
}

// BootedInTabletMode returns a precondition similar to Booted(). The only difference from Booted() is
// that Chrome is launched in tablet mode in this precondition.
func BootedInTabletMode() testing.Precondition { return bootedInTabletModePre }

// bootedInTabletModePre is returned by BootedInTabletMode.
var bootedInTabletModePre = &preImpl{
	name:      "arc_booted_in_tablet_mode",
	timeout:   resetTimeout + chrome.LoginTimeout + BootTimeout,
	extraArgs: []string{"--force-tablet-mode=touch_view", "--enable-virtual-keyboard"},
}

// VMBootedInTabletMode returns a precondition similar to BootedInTabletMode().
// The only difference from BootedInTabletMode() is that Chrome is launched in tablet mode in this precondition.
func VMBootedInTabletMode() testing.Precondition { return vmBootedInTabletModePre }

// vmBootedInTabletModePre is returned by VMBootedInTabletMode.
var vmBootedInTabletModePre = &preImpl{
	name:      "arcvm_booted_in_tablet_mode",
	timeout:   resetTimeout + chrome.LoginTimeout + BootTimeout,
	extraArgs: []string{"--enable-arcvm", "--force-tablet-mode=touch_view", "--enable-virtual-keyboard"},
}

// BootedWithVideoLogging returns a precondition similar to Booted(), but with additional Chrome video logging enabled.
func BootedWithVideoLogging() testing.Precondition { return bootedWithVideoLoggingPre }

// bootedWithVideoLoggingPre is returned by BootedWithVideoLogging.
var bootedWithVideoLoggingPre = &preImpl{
	name:    "arc_booted_with_video_logging",
	timeout: resetTimeout + chrome.LoginTimeout + BootTimeout,
	extraArgs: []string{
		"--vmodule=" + strings.Join([]string{
			"*/media/gpu/chromeos/*=2",
			"*/media/gpu/vaapi/*==2",
			"*/media/gpu/v4l2/*=2",
			"*/components/arc/video_accelerator/*"}, ",")},
}

// BootedAppCompat returns a precondition similar to Booted(). The only difference from Booted() is
// that it will GAIA login with the app compat credentials, and opt-in to the Play Store.
func BootedAppCompat() testing.Precondition { return bootedAppCompatPre }

// bootedAppCompatPre is returned by BootedAppCompat.
var bootedAppCompatPre = &preImpl{
	name:    "arc_booted_appcompat",
	timeout: resetTimeout + chrome.LoginTimeout + BootTimeout + optin.OptinTimeout,
	gaia: &gaiaVars{
		userVar: "arcappcompat.username",
		passVar: "arcappcompat.password",
	},
	extraArgs: []string{"--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off"},
}

// VMBootedAppCompat returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that ARC VM, and not the ARC Container, is enabled in this precondition.
func VMBootedAppCompat() testing.Precondition { return vmBootedAppCompatPre }

// vmBootedAppCompatPre is returned by VMBootedAppCompat.
var vmBootedAppCompatPre = &preImpl{
	name:    "arcvm_booted_appcompat",
	timeout: resetTimeout + chrome.LoginTimeout + BootTimeout + optin.OptinTimeout,
	gaia: &gaiaVars{
		userVar: "arcappcompat.username",
		passVar: "arcappcompat.password",
	},
	extraArgs: []string{"--enable-arcvm", "--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off"},
}

// BootedInTabletModeAppCompat returns a precondition similar to BootedAppCompat(). The only difference from BootedAppCompat() is
// that Chrome is launched in tablet mode in this precondition.
func BootedInTabletModeAppCompat() testing.Precondition { return bootedInTabletModeAppCompatPre }

// bootedInTabletModeAppCompatPre is returned by BootedInTabletModeAppCompat.
var bootedInTabletModeAppCompatPre = &preImpl{
	name:    "arc_booted_in_tablet_mode_appcompat",
	timeout: resetTimeout + chrome.LoginTimeout + BootTimeout + optin.OptinTimeout,
	gaia: &gaiaVars{
		userVar: "arcappcompat.username",
		passVar: "arcappcompat.password",
	},
	extraArgs: []string{"--force-tablet-mode=touch_view", "--enable-virtual-keyboard", "--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off"},
}

// VMBootedInTabletModeAppCompat returns a precondition similar to BootedInTabletModeAppCompat(). The only difference from BootedInTabletModeAppCompat() is
// that ARC VM, and not the ARC Container, is enabled in this precondition.
func VMBootedInTabletModeAppCompat() testing.Precondition { return vmBootedInTabletModeAppCompatPre }

// vmBootedInTabletModeAppCompatPre is returned by VMBootedInTabletModeAppCompat.
var vmBootedInTabletModeAppCompatPre = &preImpl{
	name:    "arcvm_booted_in_tablet_mode_appcompat",
	timeout: resetTimeout + chrome.LoginTimeout + BootTimeout + optin.OptinTimeout,
	gaia: &gaiaVars{
		userVar: "arcappcompat.username",
		passVar: "arcappcompat.password",
	},
	extraArgs: []string{"--enable-arcvm", "--force-tablet-mode=touch_view", "--enable-virtual-keyboard", "--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off"},
}

// gaiaVars holds the secret variables for username and password for a GAIA login.
type gaiaVars struct {
	userVar string // the secret variable for the GAIA username
	passVar string // the secret variable for the GAIA password
}

// preImpl implements both testing.Precondition and testing.preconditionImpl.
type preImpl struct {
	name    string        // testing.PreconditionImpl.String
	timeout time.Duration // testing.PreconditionImpl.Timeout

	extraArgs []string  // passed to Chrome on initialization
	gaia      *gaiaVars // a struct containing GAIA secret variables

	cr  *chrome.Chrome
	arc *ARC

	origInitPID       int32               // initial PID (outside container) of ARC init process
	origInstalledPkgs map[string]struct{} // initially-installed packages
	origRunningPkgs   map[string]struct{} // initially-running packages
}

func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

// Prepare is called by the test framework at the beginning of every test using this precondition.
// It returns a PreData containing objects that can be used by the test.
func (p *preImpl) Prepare(ctx context.Context, s *testing.State) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	if p.arc != nil {
		pre, err := func() (interface{}, error) {
			ctx, cancel := context.WithTimeout(ctx, resetTimeout)
			defer cancel()
			ctx, st := timing.Start(ctx, "reset_"+p.name)
			defer st.End()
			installed, err := p.arc.InstalledPackages(ctx)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get installed packages")
			}
			running, err := p.runningPackages(ctx)
			if err != nil {
				return nil, errors.Wrap(err, "failed to get running packages")
			}
			if err := p.checkUsable(ctx, installed, running); err != nil {
				return nil, errors.Wrap(err, "existing Chrome or ARC connection is unusable")
			}
			if err := p.resetState(ctx, installed, running); err != nil {
				return nil, errors.Wrap(err, "failed resetting existing Chrome or ARC session")
			}
			if err := p.arc.setLogcatFile(filepath.Join(s.OutDir(), logcatName)); err != nil {
				return nil, errors.Wrap(err, "failed to update logcat output file")
			}
			return PreData{p.cr, p.arc}, nil
		}()
		if err == nil {
			s.Log("Reusing existing ARC session")
			return pre
		}
		s.Log("Failed to reuse existing ARC session: ", err)
		locked = false
		chrome.Unlock()
		p.closeInternal(ctx, s)
	}

	// Revert partial initialization.
	shouldClose := true
	defer func() {
		if shouldClose {
			p.closeInternal(ctx, s)
		}
	}()

	func() {
		ctx, cancel := context.WithTimeout(ctx, chrome.LoginTimeout)
		defer cancel()
		var err error
		if p.gaia != nil {
			username := s.RequiredVar(p.gaia.userVar)
			password := s.RequiredVar(p.gaia.passVar)
			p.cr, err = chrome.New(ctx, chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"), chrome.ARCSupported(), chrome.ExtraArgs(p.extraArgs...))
		} else {
			p.cr, err = chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs(p.extraArgs...))
		}
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
	}()

	// Check whether ARCVM status is consistent with the precondition name.
	// We assume here that the preconditions specifying ARC Container have names starting from "arc_",
	// whereas those specifying ARCVM have names starting from "arcvm_".
	vm, err := VMEnabled()
	if err != nil {
		s.Fatal("Failed to check whether ARCVM is enabled: ", err)
	}
	if vm {
		const vmPrefix = "arcvm_"
		if !strings.HasPrefix(p.name, vmPrefix) {
			s.Fatalf("Running in ARCVM mode, but the precondition name %q does not start from %q", p.name, vmPrefix)
		}
	} else {
		const containerPrefix = "arc_"
		if !strings.HasPrefix(p.name, containerPrefix) {
			s.Fatalf("Running in ARC Container mode, but the precondition name %q does not start from %q", p.name, containerPrefix)
		}
	}

	// Opt-in if performing a GAIA login.
	if p.gaia != nil {
		func() {
			ctx, cancel := context.WithTimeout(ctx, optin.OptinTimeout)
			defer cancel()
			tconn, err := p.cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create test API connection: ", err)
			}
			if err := optin.Perform(ctx, p.cr, tconn); err != nil {
				s.Fatal("Failed to opt-in to Play Store: ", err)
			}
			if err := optin.WaitForPlayStoreShown(ctx, tconn); err != nil {
				s.Fatal("Failed to wait for Play Store: ", err)
			}
			if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
				s.Fatal("Failed to close Play Store: ", err)
			}
		}()
	}

	func() {
		ctx, cancel := context.WithTimeout(ctx, BootTimeout)
		defer cancel()
		var err error
		if p.arc, err = New(ctx, s.OutDir()); err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
		if p.origInitPID, err = InitPID(); err != nil {
			s.Fatal("Failed to get initial init PID: ", err)
		}
		if p.origInstalledPkgs, err = p.arc.InstalledPackages(ctx); err != nil {
			s.Fatal("Failed to list initial packages: ", err)
		}
		if p.origRunningPkgs, err = p.runningPackages(ctx); err != nil {
			s.Fatal("Failed to list running packages: ", err)
		}
	}()

	// Prevent the arc and chrome package's New and Close functions from
	// being called while this precondition is active.
	locked = true
	chrome.Lock()

	shouldClose = false
	return PreData{p.cr, p.arc}
}

// Close is called by the test framework after the last test that uses this precondition.
func (p *preImpl) Close(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	locked = false
	chrome.Unlock()
	p.closeInternal(ctx, s)
}

// runningPackages returns a set of currently-running packages, e.g. "com.android.settings".
// It queries all running activities, but it returns the activity's package name.
func (p *preImpl) runningPackages(ctx context.Context) (map[string]struct{}, error) {
	tasks, err := p.arc.DumpsysActivityActivities(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "listing activities failed")
	}

	acts := make(map[string]struct{})
	for _, t := range tasks {
		for _, a := range t.ActivityInfos {
			acts[a.PackageName] = struct{}{}
		}
	}
	return acts, nil
}

// checkUsable verifies that p.cr and p.arc are still usable. Both must be non-nil.
// installed should come from InstalledPackages.
// running should come from runningPackages.
func (p *preImpl) checkUsable(ctx context.Context, installed, running map[string]struct{}) error {
	ctx, st := timing.Start(ctx, "check_arc")
	defer st.End()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Check that the init process is the same as before. Otherwise, ARC was probably restarted.
	if pid, err := InitPID(); err != nil {
		return err
	} else if pid != p.origInitPID {
		return errors.Errorf("init process changed from %v to %v; probably crashed", p.origInitPID, pid)
	}

	// Check that the package manager service is running.
	const pkgi = "android"
	if _, ok := installed[pkgi]; !ok {
		return errors.Errorf("pm didn't list %q among %d package(s)", pkgi, len(installed))
	}

	// Check that home package is running.
	const pkgr = "org.chromium.arc.home"
	if _, ok := running[pkgr]; !ok {
		return errors.Errorf("package %q is not running", pkgr)
	}

	// TODO(nya): Should we also check that p.cr is still usable?
	return nil
}

// resetState resets ARC's and Chrome's state between tests.
// installed should come from InstalledPackages.
// running should come from runningPackages.
func (p *preImpl) resetState(ctx context.Context, installed, running map[string]struct{}) error {
	// Stop any packages that weren't present when ARC booted. Stop before uninstall.
	for pkg := range running {
		if _, ok := p.origRunningPkgs[pkg]; ok {
			continue
		}
		testing.ContextLogf(ctx, "Stopping package %q", pkg)
		if err := p.arc.Command(ctx, "am", "force-stop", pkg).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to stop %q", pkg)
		}
	}

	// Uninstall any packages that weren't present when ARC booted.
	for pkg := range installed {
		if _, ok := p.origInstalledPkgs[pkg]; ok {
			continue
		}
		testing.ContextLog(ctx, "Uninstalling ", pkg)
		if err := p.arc.Command(ctx, "pm", "uninstall", pkg).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to uninstall %v", pkg)
		}
	}

	if err := p.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting Chrome's state")
	}
	return nil
}

// closeInternal closes and resets p.arc and p.cr if non-nil.
func (p *preImpl) closeInternal(ctx context.Context, s *testing.State) {
	if p.arc != nil {
		if err := p.arc.Close(); err != nil {
			s.Log("Failed to close ARC connection: ", err)
		}
		p.arc = nil
	}
	p.origInstalledPkgs = nil
	p.origRunningPkgs = nil

	if p.cr != nil {
		if err := p.cr.Close(ctx); err != nil {
			s.Log("Failed to close Chrome connection: ", err)
		}
		p.cr = nil
	}
}

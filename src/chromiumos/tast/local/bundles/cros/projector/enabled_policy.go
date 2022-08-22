// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package projector is used for writing Projector tests.
package projector

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/vdi/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EnabledPolicy,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Projector app gets enable/disable when policy change",
		Contacts:     []string{"xiqiruan@chromium.org", "cros-projector+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Fixture:      fixture.FakeDMS,
	})
}

func EnabledPolicy(ctx context.Context, s *testing.State) {
	// Start FakeDMS.
	fdms, err := fakedms.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}
	defer fdms.Stop(ctx)

	subTestTimeout := 80 * time.Second
	for _, subTest := range []struct {
		name     string
		testFunc func(context.Context, *chrome.Chrome, *fakedms.FakeDMS, *chrome.TestConn) error
		policy   []policy.Policy
	}{
		{
			"testProjectorEnabledTrue",
			testProjectorEnabledTrue,
			[]policy.Policy{
				&policy.ProjectorEnabled{Val: true}},
		},
		{
			"testProjectorEnabledFalse",
			testProjectorEnabledFalse,
			[]policy.Policy{
				&policy.ProjectorEnabled{Val: false}},
		},
	} {
		subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
		s.Run(subTestCtx, subTest.name, func(ctx context.Context, s *testing.State) {
			pb := policy.NewBlob()
			pb.AddPolicies(subTest.policy)

			fdms.WritePolicyBlob(pb)
			// Starts a Chrome instance that will fetch policies from the FakeDMS.
			cr, err := chrome.New(ctx,
				chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
				chrome.DMSPolicy(fdms.URL),
				chrome.EnableFeatures("Projector, ProjectorAppDebug"),
			)

			if err != nil {
				s.Fatal("Chrome startup failed: ", err)
			}
			defer cr.Close(ctx)
			tconn, err := cr.TestAPIConn(ctx)
			defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

			if err := policyutil.Refresh(ctx, tconn); err != nil {
				s.Fatal("Failed to update Chrome policies: ", err)
			}
			if err := subTest.testFunc(ctx, cr, fdms, tconn); err != nil {
				s.Fatalf("Failed to run subtest %v: %v", subTest.name, err)
			}
		})
		cancel()
	}

}

// testProjectorEnabledTrue verifies the Screencast app is installed when user
// login for the first time with the ProjectorEnabled policy is true. It also
// verifies the app is disabled when the policy updated in session.
func testProjectorEnabledTrue(ctx context.Context, cr *chrome.Chrome, fdms *fakedms.FakeDMS, tconn *chrome.TestConn) error {
	// Verifies Screencast is in app registry.
	isScreencastRegistered := false
	registeredApp, err := apps.ListRegisteredSystemWebApps(ctx, tconn)
	if err != nil {
		return err
	}

	for _, app := range registeredApp {
		if app.Name == apps.Projector.Name {
			isScreencastRegistered = true
		}
	}

	if !isScreencastRegistered {
		return errors.New("Screencast is not in SWA registry")
	}

	// SWA installation is not guaranteed during startup.
	// Wait for installation finished before starting test.
	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.Projector.ID,
		2*time.Minute); err != nil {
		errors.Wrap(err, "failed to wait for installed app")
	}

	// Waits until the Screencast app window exists:
	if err := launcher.LaunchAndWaitForAppOpen(tconn, apps.Projector)(ctx); err != nil {
		return errors.Wrap(err, "failed to open Projector app")
	}

	ui := uiauto.New(tconn).WithTimeout(10 * time.Second)
	if err := ui.WaitUntilExists(nodewith.Name("Screencast").Role(role.Window).ClassName("BrowserFrame"))(ctx); err != nil {
		return errors.Wrap(err, "Screencast app window not found")
	}

	// Disables the policy in session and verified the app is blocked in launcher:
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{
		&policy.ProjectorEnabled{Val: false}}); err != nil {
		return errors.Wrap(err, "failed to serve policies")
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find keyboard")
	}
	defer kb.Close()

	blockedWindowFinder := nodewith.Role(role.Window).Name("Screencast is blocked")
	if err := launcher.SearchAndLaunch(tconn, kb, apps.Projector.Name)(ctx); err != nil {
		return errors.Wrap(err, "failed to find Screencast in the launcher")
	}

	if err = ui.WaitUntilExists(blockedWindowFinder)(ctx); err != nil {
		return errors.Wrap(err, "failed to check and close blocked window")
	}

	if err = kb.Accel(ctx, "Enter"); err != nil {
		return errors.Wrap(err, "failed to press Enter to close Screencast warning dialog")
	}

	if err = ui.WaitUntilGone(blockedWindowFinder)(ctx); err != nil {
		return errors.Wrap(err, "failed to close Screencast warning dialog. This might potentially effect later tests")
	}
	return nil
}

// testProjectorEnabledFalse verifies the Screencast app is not installed when
// user login for the first time with the ProjectorEnabled policy is false.
func testProjectorEnabledFalse(ctx context.Context, _ *chrome.Chrome, _ *fakedms.FakeDMS, tconn *chrome.TestConn) error {
	if installed, err := ash.ChromeAppInstalled(ctx, tconn, apps.Projector.ID); err != nil {
		return errors.Wrap(err, "failed to get Screencast app install status")
	} else if installed {
		return errors.New("screencast is installed even the policy is disabled")
	}

	// Verifies Screencast is not in app registry.
	registeredApp, err := apps.ListRegisteredSystemWebApps(ctx, tconn)
	if err != nil {
		return err
	}
	for _, app := range registeredApp {
		if app.Name == apps.Projector.Name {
			return errors.New("screencast is in SWA registry while policy is diabled")
		}
	}
	return nil
}

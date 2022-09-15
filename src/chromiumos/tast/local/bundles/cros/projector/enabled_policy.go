// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/vdi/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EnabledPolicy,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Projector app gets enabled/disabled when the policy changes",
		Contacts:     []string{"xiqiruan@chromium.org", "cros-projector+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Fixture:      fixture.FakeDMS,
	})
}

func EnabledPolicy(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	// Starts FakeDMS.
	fdms, err := fakedms.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}

	defer fdms.Stop(ctxForCleanUp)

	subTestTimeout := 3 * time.Minute
	for _, subTest := range []struct {
		name     string
		testFunc func(context.Context, *chrome.Chrome, *fakedms.FakeDMS, *chrome.TestConn) error
		policy   []policy.Policy
	}{
		{
			"testProjectorEnabled",
			testProjectorEnabled,
			[]policy.Policy{
				&policy.ProjectorEnabled{Val: true}},
		},
		{
			"testProjectorDisabled",
			testProjectorDisabled,
			[]policy.Policy{
				&policy.ProjectorEnabled{Val: false}},
		},
	} {
		subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
		s.Run(subTestCtx, subTest.name, func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

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
			defer policyutil.ResetChrome(cleanupCtx, fdms, cr)
			defer cr.Close(cleanupCtx)
			tconn, err := cr.TestAPIConn(ctx)
			defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

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

// testProjectorEnabled verifies the Screencast app is installed when user
// login for the first time with the ProjectorEnabled policy is true. It also
// verifies the app is disabled when the policy updated in session.
func testProjectorEnabled(ctx context.Context, cr *chrome.Chrome, fdms *fakedms.FakeDMS, tconn *chrome.TestConn) error {
	// Verifies Screencast is in SWA registry.
	isScreencastRegistered := false
	registeredApp, err := apps.ListRegisteredSystemWebApps(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to list registered apps")
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
		return errors.Wrap(err, "failed to wait for installed app")
	}

	// Waits until the Screencast app window exists:
	if err := launcher.LaunchAndWaitForAppOpen(tconn, apps.Projector)(ctx); err != nil {
		return errors.Wrap(err, "failed to open Projector app")
	}

	ui := uiauto.New(tconn).WithTimeout(10 * time.Second)
	screencastAppWindow := nodewith.Name("Screencast").Role(role.Window).ClassName("BrowserFrame")
	if err := ui.WaitUntilExists(screencastAppWindow)(ctx); err != nil {
		return errors.Wrap(err, "Screencast app window not found")
	}

	// Disables the policy in session and verified the app is blocked in launcher:
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{
		&policy.ProjectorEnabled{Val: false}}); err != nil {
		return errors.Wrap(err, "failed to serve policies")
	}

	if err := uiauto.Combine("Wait for the app and launcher to close",
		ui.WaitUntilGone(screencastAppWindow),
		// Ensures the app list view is closed before open it and launch app again.
		ui.WaitUntilGone(nodewith.ClassName(launcher.ExpandedItemsClass).First()),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for for Screencast and launcher to close")
	}

	if err := launcher.LaunchApp(tconn, apps.Projector.Name)(ctx); err != nil {
		return errors.Wrap(err, "failed to find Screencast in the launcher")
	}

	blockedWindowFinder := nodewith.Role(role.Window).Name("Screencast is blocked")
	okButton := nodewith.Role(role.Button).Name("OK").Ancestor(blockedWindowFinder)
	if err := uiauto.Combine("Confirm Screencast is blocked",
		ui.WaitUntilExists(blockedWindowFinder),
		ui.WithInterval(time.Second).LeftClickUntil(okButton, ui.Gone(blockedWindowFinder)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to verify the screencast is blocked when policy is disabled")
	}
	return nil
}

// testProjectorDisabled verifies the Screencast app is not installed when
// user login for the first time with the ProjectorEnabled policy is false.
func testProjectorDisabled(ctx context.Context, _ *chrome.Chrome, _ *fakedms.FakeDMS, tconn *chrome.TestConn) error {
	// Verifies Screencast is not in SWA registry.
	registeredApp, err := apps.ListRegisteredSystemWebApps(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to list registered apps")
	}
	for _, app := range registeredApp {
		if app.Name == apps.Projector.Name {
			return errors.New("Screencast App is in SWA registry while policy is disabled")
		}
	}
	return nil
}

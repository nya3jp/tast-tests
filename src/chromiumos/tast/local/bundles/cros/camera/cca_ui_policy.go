// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIPolicy,
		Desc:         "Verifies if CCA is unusable when the camera app is disabled by the Adenterprise policy",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
	})
}

func applyPolicy(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, ps []policy.Policy) error {
	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		return errors.Wrap(err, "failed to reset Chrome")
	}

	if err := policyutil.ServeAndVerify(ctx, fdms, cr, ps); err != nil {
		return errors.Wrap(err, "failed to apply policy")
	}
	return nil
}

func CCAUIPolicy(ctx context.Context, s *testing.State) {
	fdms, err := fakedms.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}
	pb := fakedms.NewPolicyBlob()
	if err := fdms.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}
	cr, err := chrome.New(ctx,
		chrome.Auth("tast-user@managedchrome.com", "test0000", "gaia-id"),
		chrome.DMSPolicy(fdms.URL))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	cleanup := func(ctx context.Context, app *cca.App) {
		if app != nil {
			app.Close(ctx)
		}
	}

	// Test: Without any policy, CCA works fine
	if err := applyPolicy(ctx, fdms, cr, []policy.Policy{}); err != nil {
		s.Fatal("Failed to apply policy: ", err)
	}
	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start CCA with no specific policy")
	}
	defer cleanup(ctx, app)

	// Test: Block CCA extension.
	// Expect: CCA will not be installed.
	if err := applyPolicy(ctx, fdms, cr, []policy.Policy{&policy.ExtensionInstallBlacklist{Val: []string{cca.ID}}}); err != nil {
		s.Fatal("Failed to apply policy: ", err)
	}
	if exist, err := cr.IsTargetAvailable(ctx, chrome.MatchTargetURL(cca.BackgroundURL)); err != nil {
		s.Fatal("Failed to check if CCA is installed")
	} else if exist {
		s.Fatal("Failed to block CCA by policy")
	}

	// Test: Block camera.
	// Expect: A message box "Camera is blocked" will show when launching CCA through the launcher.
	if err := applyPolicy(ctx, fdms, cr, []policy.Policy{&policy.SystemFeaturesDisableList{Val: []string{"camera"}}}); err != nil {
		s.Fatal("Failed to apply policy: ", err)
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test extension connection: ", err)
	}
	if err := launcher.SearchAndLaunch(ctx, tconn, "Camera"); err != nil {
		s.Fatal("Failed to find camera app in the launcher: ", err)
	}
	_, err = ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "BubbleDialogDelegateView", Name: "Camera is blocked"}, 5*time.Second)
	if err != nil {
		s.Fatal("Failed to get blocked dialog: ", err)
	}

	// Test: Block video capture.
	// Expect: CCA fails to initialize since it is launched but preview won't show.
	if err := applyPolicy(ctx, fdms, cr, []policy.Policy{&policy.VideoCaptureAllowed{Val: false}}); err != nil {
		s.Fatal("Failed to apply policy: ", err)
	}
	app, err = cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir())
	if err == nil {
		s.Fatal("Failed to block video capture by policy")
	} else if !errors.Is(err, cca.ErrVideoNotActive) {
		s.Fatal("Unexpected error when blocking video capture: ", err)
	}
	defer cleanup(ctx, app)
}

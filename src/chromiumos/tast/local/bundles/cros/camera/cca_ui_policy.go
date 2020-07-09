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

func CCAUIPolicy(ctx context.Context, s *testing.State) {
	fdms, err := fakedms.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}
	if err := fdms.WritePolicyBlob(fakedms.NewPolicyBlob()); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}
	cr, err := chrome.New(ctx,
		chrome.Auth("tast-user@managedchrome.com", "test0000", "gaia-id"),
		chrome.DMSPolicy(fdms.URL))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	if err := testNoPolicy(ctx, fdms, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir()); err != nil {
		s.Error("Failed to test with no policy: ", err)
	}

	if err := testBlockCCAExtension(ctx, fdms, cr); err != nil {
		s.Error("Failed to block CCA extension: ", err)
	}

	if err := testBlockCameraFeature(ctx, fdms, cr); err != nil {
		s.Error("Failed to block camera feature: ", err)
	}

	if err := testBlockVideoCapture(ctx, fdms, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir()); err != nil {
		s.Error("Failed to block video capture: ", err)
	}
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

// testNoPolicy tests without any policy and expects CCA works fine.
func testNoPolicy(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, scripts []string, outDir string) error {
	if err := applyPolicy(ctx, fdms, cr, []policy.Policy{}); err != nil {
		return errors.Wrap(err, "failed to apply policy")
	}
	app, err := cca.New(ctx, cr, scripts, outDir)
	if err != nil {
		return errors.Wrap(err, "failed to start CCA with no policy")
	}
	return app.Close(ctx)
}

// testBlockCCAExtension tries to block CCA extension and expects CCA will not
// be installed.
func testBlockCCAExtension(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome) error {
	if err := applyPolicy(ctx, fdms, cr, []policy.Policy{&policy.ExtensionInstallBlacklist{Val: []string{cca.ID}}}); err != nil {
		return err
	}
	if available, err := cr.IsTargetAvailable(ctx, chrome.MatchTargetURL(cca.BackgroundURL)); err != nil {
		return errors.Wrap(err, "failed to check if CCA is installed")
	} else if available {
		return errors.New("failed to block CCA by policy")
	}
	return nil
}

// testBlockCameraFeature tries to block camera feature and expects a message
// box "Camera is blocked" will show when launching CCA through the launcher.
func testBlockCameraFeature(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome) error {
	if err := applyPolicy(ctx, fdms, cr, []policy.Policy{&policy.SystemFeaturesDisableList{Val: []string{"camera"}}}); err != nil {
		return errors.Wrap(err, "failed to apply policy")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get test extension connection")
	}
	if err := launcher.SearchAndLaunch(ctx, tconn, "Camera"); err != nil {
		return errors.Wrap(err, "failed to find camera app in the launcher")
	}
	_, err = ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "BubbleDialogDelegateView", Name: "Camera is blocked"}, 5*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to get blocked dialog")
	}
	return nil
}

// testBlockVideoCapture tries to block video capture and expects CCA fails to
// initialize since the preview won't show.
func testBlockVideoCapture(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, scripts []string, outDir string) error {
	if err := applyPolicy(ctx, fdms, cr, []policy.Policy{&policy.VideoCaptureAllowed{Val: false}}); err != nil {
		return errors.Wrap(err, "failed to apply policy")
	}
	app, err := cca.New(ctx, cr, scripts, outDir)
	if !errors.Is(err, cca.ErrVideoNotActive) {
		if err == nil {
			if err := app.Close(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to close app: ", err)
			}
			return errors.New("failed to block video capture by policy")
		}
		return errors.Wrap(err, "unexpected error when blocking video capture")
	}
	return nil
}

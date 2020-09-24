// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/bundles/cros/camera/testutil"
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
		Pre: testutil.NewPrecondition(testutil.ChromeConfig{
			UseFakeDMS: true,
		}),
	})
}

func CCAUIPolicy(ctx context.Context, s *testing.State) {
	p := s.PreValue().(testutil.PreData)
	cr := p.Chrome
	fdms := p.FakeDMS
	tb := p.TestBridge
	isSWA := p.Config.InstallSWA
	scripts := []string{s.DataPath("cca_ui.js")}
	outDir := s.OutDir()

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	if err := testNoPolicy(ctx, fdms, cr, scripts, outDir, tb, isSWA); err != nil {
		if cca.IsJSError(err) {
			s.Error("There are JS errors when running CCA: ", err)
		} else {
			s.Error("Failed to test with no policy: ", err)
		}
	}

	if err := testBlockCCAExtension(ctx, fdms, cr, tb); err != nil {
		s.Error("Failed to block CCA extension: ", err)
	}

	if err := testBlockCameraFeature(ctx, fdms, cr, tb); err != nil {
		s.Error("Failed to block camera feature: ", err)
	}

	if err := testBlockVideoCapture(ctx, fdms, cr, scripts, outDir, tb, isSWA); err != nil {
		if cca.IsJSError(err) {
			s.Error("There are JS errors when running CCA: ", err)
		} else {
			s.Error("Failed to block video capture: ", err)
		}
	}
}

func servePolicy(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, ps []policy.Policy, tb *testutil.TestBridge) error {
	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		return errors.Wrap(err, "failed to reset Chrome")
	}

	if err := policyutil.ServeAndVerify(ctx, fdms, cr, ps); err != nil {
		return errors.Wrap(err, "failed to serve policy")
	}

	// Since we have reset Chrome, we should reset the test bridge so that we
	// can communicate with CCA again in the following tests.
	if err := tb.Reset(ctx); err != nil {
		return errors.Wrap(err, "failed to reset test bridge")
	}

	return nil
}

// testNoPolicy tests without any policy and expects CCA works fine.
func testNoPolicy(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, scripts []string, outDir string, tb *testutil.TestBridge, isSWA bool) error {
	if err := servePolicy(ctx, fdms, cr, []policy.Policy{}, tb); err != nil {
		return errors.Wrap(err, "failed to serve policy")
	}

	app, err := cca.New(ctx, cr, scripts, outDir, tb, isSWA)
	if err != nil {
		return errors.Wrap(err, "failed to start CCA with no policy")
	}
	return app.Close(ctx)
}

// testBlockCCAExtension tries to block CCA extension and expects the background
// page of CCA is not accessible.
func testBlockCCAExtension(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, tb *testutil.TestBridge) error {
	if err := servePolicy(ctx, fdms, cr, []policy.Policy{&policy.ExtensionInstallBlacklist{Val: []string{cca.ID}}}, tb); err != nil {
		return err
	}
	if available, err := cr.IsTargetAvailable(ctx, chrome.MatchTargetURL(testutil.BackgroundURL)); err != nil {
		return errors.Wrap(err, "failed to check if CCA is installed")
	} else if available {
		return errors.New("failed to block CCA by policy")
	}
	return nil
}

// testBlockCameraFeature tries to block camera feature and expects a message
// box "Camera is blocked" will show when launching CCA through the launcher.
func testBlockCameraFeature(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, tb *testutil.TestBridge) error {
	if err := servePolicy(ctx, fdms, cr, []policy.Policy{&policy.SystemFeaturesDisableList{Val: []string{"camera"}}}, tb); err != nil {
		return errors.Wrap(err, "failed to serve policy")
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
func testBlockVideoCapture(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, scripts []string, outDir string, tb *testutil.TestBridge, isSWA bool) error {
	if err := servePolicy(ctx, fdms, cr, []policy.Policy{&policy.VideoCaptureAllowed{Val: false}}, tb); err != nil {
		return errors.Wrap(err, "failed to serve policy")
	}

	app, err := cca.New(ctx, cr, scripts, outDir, tb, isSWA)
	if err == nil {
		if err := app.Close(ctx); err != nil && !cca.IsJSError(err) {
			// It is acceptable that there are errors in CCA since the video
			// capture is blocked. Reports if the error is not JS error.
			testing.ContextLog(ctx, "Failed to close app: ", err)
		}
		return errors.New("failed to block video capture by policy")
	} else if !strings.Contains(err.Error(), cca.ErrVideoNotActive) {
		return errors.Wrap(err, "unexpected error when blocking video capture")
	}
	return nil
}

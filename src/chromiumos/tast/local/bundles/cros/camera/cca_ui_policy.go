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
	})
}

func CCAUIPolicy(ctx context.Context, s *testing.State) {
	fdms, err := fakedms.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}
	defer fdms.Stop(ctx)

	if err := fdms.WritePolicyBlob(fakedms.NewPolicyBlob()); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	opts := []chrome.Option{
		chrome.Auth("tast-user@managedchrome.com", "test0000", "gaia-id"),
		chrome.DMSPolicy(fdms.URL)}
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tb, err := testutil.NewTestBridge(ctx, cr)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	scripts := []string{s.DataPath("cca_ui.js")}
	outDir := s.OutDir()

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	if err := testNoPolicy(ctx, fdms, cr, scripts, outDir, tb); err != nil {
		s.Error("Failed to test with no policy: ", err)
	}

	if err := testBlockCameraFeature(ctx, fdms, cr, tb); err != nil {
		s.Error("Failed to block camera feature: ", err)
	}

	if err := testBlockVideoCapture(ctx, fdms, cr, scripts, outDir, tb); err != nil {
		s.Error("Failed to block video capture: ", err)
	}
}

func servePolicy(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, ps []policy.Policy, tb *testutil.TestBridge, shouldReconstructTestBridge bool) (retErr error) {
	if err := tb.TearDown(ctx); err != nil {
		return errors.Wrap(err, "failed to tear down the test bridge before resetting Chrome")
	}

	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		return errors.Wrap(err, "failed to reset Chrome")
	}
	// Since we have reset Chrome, we should reset the test bridge so that we
	// can communicate with CCA again in the following tests, except for
	// extension blocking tests.
	defer func(ctx context.Context) {
		if shouldReconstructTestBridge {
			if err := tb.Reset(ctx); err != nil {
				if retErr != nil {
					testing.ContextLog(ctx, "Failed to reset test bridge: ", err)
				} else {
					retErr = err
				}
			}
		}
	}(ctx)

	if err := policyutil.ServeAndVerify(ctx, fdms, cr, ps); err != nil {
		return errors.Wrap(err, "failed to serve policy")
	}
	return nil
}

// testNoPolicy tests without any policy and expects CCA works fine.
func testNoPolicy(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, scripts []string, outDir string, tb *testutil.TestBridge) error {
	if err := servePolicy(ctx, fdms, cr, []policy.Policy{}, tb, true); err != nil {
		return errors.Wrap(err, "failed to serve policy")
	}

	app, err := cca.New(ctx, cr, scripts, outDir, tb)
	if err != nil {
		return errors.Wrap(err, "failed to start CCA with no policy")
	}
	return app.Close(ctx)
}

// testBlockCameraFeature tries to block camera feature and expects a message
// box "Camera is blocked" will show when launching CCA through the launcher.
func testBlockCameraFeature(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, tb *testutil.TestBridge) error {
	if err := servePolicy(ctx, fdms, cr, []policy.Policy{&policy.SystemFeaturesDisableList{Val: []string{"camera"}}}, tb, true); err != nil {
		return errors.Wrap(err, "failed to serve policy")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get test extension connection")
	}
	if err := launcher.SearchAndLaunch(ctx, tconn, "Camera"); err != nil {
		return errors.Wrap(err, "failed to find camera app in the launcher")
	}
	dialogView, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "BubbleDialogDelegateView", Name: "Camera is blocked"}, 5*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to get blocked dialog")
	}
	defer dialogView.Release(ctx)

	return nil
}

// testBlockVideoCapture tries to block video capture and expects CCA fails to
// initialize since the preview won't show.
func testBlockVideoCapture(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, scripts []string, outDir string, tb *testutil.TestBridge) error {
	if err := servePolicy(ctx, fdms, cr, []policy.Policy{&policy.VideoCaptureAllowed{Val: false}}, tb, true); err != nil {
		return errors.Wrap(err, "failed to serve policy")
	}

	app, err := cca.New(ctx, cr, scripts, outDir, tb)
	if err == nil {
		var errJS *cca.ErrJS
		if err := app.Close(ctx); err != nil && !errors.As(err, &errJS) {
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

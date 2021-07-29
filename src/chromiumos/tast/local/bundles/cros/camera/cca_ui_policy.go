// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIPolicy,
		Desc:         "Verifies if CCA is unusable when the camera app is disabled by the Adenterprise policy",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
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
		chrome.FakeLogin(chrome.Creds{User: "tast-user@managedchrome.com", Pass: "test0000"}),
		chrome.DMSPolicy(fdms.URL)}
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	scripts := []string{s.DataPath("cca_ui.js")}
	outDir := s.OutDir()

	subTestTimeout := 20 * time.Second
	for _, tst := range []struct {
		name     string
		testFunc func(context.Context, *chrome.Chrome, []string, string) error
		policy   []policy.Policy
	}{{
		"testNoPolicy",
		testNoPolicy,
		[]policy.Policy{},
	}, {
		"testBlockCameraFeature",
		testBlockCameraFeature,
		[]policy.Policy{&policy.SystemFeaturesDisableList{Val: []string{"camera"}}},
	}, {
		"testBlockVideoCapture",
		testBlockVideoCapture,
		[]policy.Policy{&policy.VideoCaptureAllowed{Val: false}},
	}} {
		subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
		s.Run(subTestCtx, tst.name, func(ctx context.Context, s *testing.State) {
			if err := cca.ClearSavedDirs(ctx, cr); err != nil {
				s.Fatal("Failed to clear saved directory: ", err)
			}

			if err := servePolicy(ctx, fdms, cr, tst.policy); err != nil {
				s.Fatal("Failed to serve policy: ", err)
			}

			if err := tst.testFunc(ctx, cr, scripts, outDir); err != nil {
				s.Fatalf("Failed to run subtest %v: %v", tst.name, err)
			}
		})
		cancel()
	}
}

func servePolicy(ctx context.Context, fdms *fakedms.FakeDMS, cr *chrome.Chrome, ps []policy.Policy) (retErr error) {
	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		return errors.Wrap(err, "failed to reset Chrome")
	}

	if err := policyutil.ServeAndVerify(ctx, fdms, cr, ps); err != nil {
		return errors.Wrap(err, "failed to serve policy")
	}
	return nil
}

// testNoPolicy tests without any policy and expects CCA works fine.
func testNoPolicy(ctx context.Context, cr *chrome.Chrome, scripts []string, outDir string) error {
	tb, err := testutil.NewTestBridge(ctx, cr, testutil.UseRealCamera)
	if err != nil {
		return errors.Wrap(err, "failed to construct test bridge")
	}
	defer tb.TearDown(ctx)

	app, err := cca.New(ctx, cr, scripts, outDir, tb)
	if err != nil {
		return errors.Wrap(err, "failed to start CCA with no policy")
	}
	return app.Close(ctx)
}

// testBlockCameraFeature tries to block camera feature and expects a message
// box "Camera is blocked" will show when launching CCA through the launcher.
func testBlockCameraFeature(ctx context.Context, cr *chrome.Chrome, scripts []string, outDir string) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get test extension connection")
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find keyboard")
	}
	defer kb.Close()
	if err := launcher.SearchAndLaunch(tconn, kb, apps.Camera.Name)(ctx); err != nil {
		return errors.Wrap(err, "failed to find camera app in the launcher")
	}

	ui := uiauto.New(tconn).WithTimeout(10 * time.Second)
	blockedWindowFinder := nodewith.Role(role.Window).Name("Camera is blocked")
	okButtonFinder := nodewith.Role(role.Button).Name("OK")
	err = uiauto.Combine("Check and close blocked window",
		ui.WaitUntilExists(blockedWindowFinder),
		ui.LeftClick(okButtonFinder))(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check and close blocked window")
	}
	return nil
}

// testBlockVideoCapture tries to block video capture and expects CCA fails to
// initialize since the preview won't show.
func testBlockVideoCapture(ctx context.Context, cr *chrome.Chrome, scripts []string, outDir string) error {
	tb, err := testutil.NewTestBridge(ctx, cr, testutil.UseRealCamera)
	if err != nil {
		return errors.Wrap(err, "failed to construct test bridge")
	}
	defer tb.TearDown(ctx)

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

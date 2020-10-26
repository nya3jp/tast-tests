// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/bundles/cros/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type ccaUISmokeParams struct {
	useFakeDeviceInChrome bool
	appType               testutil.CCAAppType
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUISmoke,
		Desc:         "Smoke test for Chrome Camera App",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"cca_ui.js"},
		Params: []testing.Param{{
			Name:              "real",
			ExtraSoftwareDeps: []string{caps.BuiltinCamera},
			Pre:               chrome.LoggedIn(),
			Val: ccaUISmokeParams{
				appType: testutil.PlatformApp,
			},
			ExtraAttr: []string{"informational"},
		}, {
			Name:              "vivid",
			ExtraSoftwareDeps: []string{caps.VividCamera},
			Pre:               chrome.LoggedIn(),
			Val: ccaUISmokeParams{
				appType: testutil.PlatformApp,
			},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "fake",
			Val: ccaUISmokeParams{
				useFakeDeviceInChrome: true,
				appType:               testutil.PlatformApp,
			},
			// TODO(crbug.com/1050732): Remove this once the unknown crash on
			// scarlet is resolved.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("scarlet")),
		}, {
			Name:              "real_swa",
			ExtraSoftwareDeps: []string{caps.BuiltinCamera},
			Pre:               testutil.ChromeWithSWA(),
			Val: ccaUISmokeParams{
				appType: testutil.SWA,
			},
			ExtraAttr: []string{"informational"},
		}, {
			Name:              "vivid_swa",
			ExtraSoftwareDeps: []string{caps.VividCamera},
			Pre:               testutil.ChromeWithSWA(),
			Val: ccaUISmokeParams{
				appType: testutil.SWA,
			},
			ExtraAttr: []string{"informational"},
		}, {
			Name: "fake_swa",
			Val: ccaUISmokeParams{
				useFakeDeviceInChrome: true,
				appType:               testutil.SWA,
			},
			ExtraAttr: []string{"informational"},
			// TODO(crbug.com/1050732): Remove this once the unknown crash on
			// scarlet is resolved.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("scarlet")),
		}},
	})
}

func CCAUISmoke(ctx context.Context, s *testing.State) {
	useSWA := s.Param().(ccaUISmokeParams).appType == testutil.SWA
	cr, ok := s.PreValue().(*chrome.Chrome)
	if !ok {
		var opts []chrome.Option
		if s.Param().(ccaUISmokeParams).useFakeDeviceInChrome {
			opts = append(opts, chrome.ExtraArgs(
				"--use-fake-ui-for-media-stream",
				// The default fps of fake device is 20, but CCA requires fps >= 24.
				// Set the fps to 30 to avoid OverconstrainedError.
				"--use-fake-device-for-media-stream=fps=30"))
		}
		if useSWA {
			opts = append(opts, chrome.EnableFeatures("CameraSystemWebApp"))
		}
		var err error
		cr, err = chrome.New(ctx, opts...)
		if err != nil {
			s.Fatal("Failed to open chrome: ", err)
		}
		defer cr.Close(ctx)
	}

	tb, err := testutil.NewTestBridge(ctx, cr, useSWA)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb, useSWA)
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			s.Error("Failed to close app: ", err)
		}
	}(ctx)
}

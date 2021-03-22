// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUISmoke,
		Desc:         "Smoke test for Chrome Camera App",
		Contacts:     []string{"inker@chromium.org", "shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "group:camera-libcamera"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"cca_ui.js"},
		Params: []testing.Param{{
			Name:              "real",
			ExtraSoftwareDeps: []string{caps.BuiltinCamera},
			Pre:               chrome.LoggedIn(),
			ExtraAttr:         []string{"informational"},
			Val:               testutil.UseRealCamera,
		}, {
			Name:              "vivid",
			ExtraSoftwareDeps: []string{caps.VividCamera},
			Pre:               chrome.LoggedIn(),
			ExtraAttr:         []string{"group:camera-postsubmit"},
			Val:               testutil.UseVividCamera,
		}, {
			Name: "fake",
			Pre:  testutil.ChromeWithFakeCamera(),
			Val:  testutil.UseFakeCamera,
		}},
	})
}

func CCAUISmoke(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tb, err := testutil.NewTestBridge(ctx, cr, s.Param().(testutil.UseCameraType))
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb)
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			s.Error("Failed to close app: ", err)
		}
	}(ctx)
}

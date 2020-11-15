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
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIMojo,
		Desc:         "Verifies that the private Mojo APIs CCA relies on work as expected",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Params: []testing.Param{{
			Pre: testutil.ChromeWithPlatformApp(),
			Val: testutil.PlatformApp,
		}, {
			Name: "swa",
			Pre:  testutil.ChromeWithSWA(),
			Val:  testutil.SWA,
		}},
	})
}

func CCAUIMojo(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	useSWA := s.Param().(testutil.CCAAppType) == testutil.SWA
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

	if err := app.CheckMojoConnection(ctx); err != nil {
		s.Fatal("Failed to construct mojo connection: ", err)
	}
}

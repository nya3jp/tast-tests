// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"time"

	// "chromiumos/tast/ctxutil"
	// "chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/bundles/cros/camera/testutil"
	"chromiumos/tast/local/chrome"
	// "chromiumos/tast/local/input"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUILayout,
		Desc:         "TODO: Opens CCA and verifies video recording related use cases",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js", "cat.y4m"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Pre: testutil.ChromeWithPlatformApp(),
			Val: testutil.PlatformApp,
		}, {
			Name: "swa",
			Pre:  testutil.ChromeWithSWAAndFakeCamera2,
			Val:  testutil.SWA,
		}},
	})
}

func CCAUILayout(ctx context.Context, s *testing.State) {
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

	if err := app.Click(ctx, cca.CloseBannerButton); err != nil {
		s.Error("Failed to close banner: ", err)
	}

	testing.Sleep(ctx, time.Second)

	img, err := app.Screenshot(ctx)
	if err != nil {
		s.Fatal("Failed to get window bound: ", err)
	}

	f, err := os.Create(filepath.Join(s.OutDir(), "outimage.png"))
	if err != nil {
		s.Fatal("Failed to open file: ", err)
	}
	defer f.Close()

	err = png.Encode(f, *img)
	if err != nil {
		s.Fatal("Failed to save to png: ", err)
	}

	// Test variation under:
	// Different window size (window.resizeTo)
	// Different aspect ratio
	// TODO(b/172340037): Test langauage (How to switch languages?)
	// Tablet and clamshell
}

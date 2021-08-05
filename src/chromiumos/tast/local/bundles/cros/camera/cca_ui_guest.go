// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIGuest,
		Desc:         "Checks camera app can be launched in guest mode",
		Contacts:     []string{"pihsun@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Fixture:      "chromeLoggedInGuest",
	})
}

func CCAUIGuest(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tb, err := testutil.NewTestBridge(ctx, cr, testutil.UseRealCamera)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	// TODO(pihsun): We should call cca.ClearSavedDir(ctx, cr) here to prevent
	// past tests from interfering this test, but currently cca.ClearSavedDir
	// doesn't work in guest mode.

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb)
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}

	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			s.Error("Failed to close app: ", err)
		}
	}(ctx)

	// TODO(pihsun): Test take a photo. Currently app.TakeSinglePhoto fails
	// because it can't find the result photo, which is located in the guest
	// ephermeral home directory.
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIRefresh,
		Desc:         "Test for checking Chrome Camera App still works after refreshing",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Fixture:      "ccaLaunched",
	})
}

func CCAUIRefresh(ctx context.Context, s *testing.State) {
	app := s.FixtValue().(cca.FixtureData).App()
	tb := s.FixtValue().(cca.FixtureData).TestBridge
	s.Log("Refreshing CCA")
	if err := app.Refresh(ctx, tb); err != nil {
		s.Fatal("Failed to complete refresh: ", err)
	}

	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal("Preview is not shown after refreshing: ", err)
	}
}

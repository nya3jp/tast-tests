// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
		Func:         CCAUIMojo,
		Desc:         "Verifies that the private Mojo APIs CCA relies on work as expected",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Fixture:      "ccaLaunched",
	})
}

func CCAUIMojo(ctx context.Context, s *testing.State) {
	app := s.FixtValue().(cca.FixtureData).App()
	if err := app.CheckMojoConnection(ctx); err != nil {
		s.Fatal("Failed to construct mojo connection: ", err)
	}
}

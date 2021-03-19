// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIGuest,
		Desc:         "Checks camera app can be launched in guest mode",
		Contacts:     []string{"pihsun@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Fixture:      "ccaLaunchedGuest",
	})
}

func CCAUIGuest(ctx context.Context, s *testing.State) {
	// TODO(pihsun): Test take a photo. Currently app.TakeSinglePhoto fails
	// because it can't find the result photo, which is located in the guest
	// ephermeral home directory.
}

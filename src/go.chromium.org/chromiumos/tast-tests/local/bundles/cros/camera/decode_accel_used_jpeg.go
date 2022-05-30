// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"go.chromium.org/chromiumos/tast-tests/common/media/caps"
	"go.chromium.org/chromiumos/tast-tests/local/bundles/cros/camera/getusermedia"
	"go.chromium.org/chromiumos/tast-tests/local/media/constants"
	"go.chromium.org/chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelUsedJPEG,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks HW decoding is used for MJPEG in GetUserMedia()",
		Contacts: []string{
			"mojahsu@chromium.org",
			"mcasas@chromium.org", // Test author.
			"chromeos-camera-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"chrome", "camera_legacy", caps.HWDecodeJPEG},
		Data:         []string{"get_user_media.html", "crowd720_25frames.mjpeg"},
	})
}

func DecodeAccelUsedJPEG(ctx context.Context, s *testing.State) {
	getusermedia.RunDecodeAccelUsedJPEG(ctx, s, "get_user_media.html", "crowd720_25frames.mjpeg", constants.RTCJPEGInitStatus, constants.RTCJPEGInitSuccess)
}

// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/getusermedia"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/constants"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DecodeAccelUsedJPEG,
		Desc: "Checks HW decoding is used for MJPEG in GetUserMedia()",
		Contacts: []string{
			"mojahsu@chromium.org",
			"mcasas@chromium.org", // Test author.
			"chromeos-camera-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "camera_legacy", caps.HWDecodeJPEG},
		Data:         []string{"get_user_media.html", "crowd720_25frames.mjpeg"},
	})
}

func DecodeAccelUsedJPEG(ctx context.Context, s *testing.State) {
	getusermedia.RunDecodeAccelUsedJPEG(ctx, s, "get_user_media.html", "crowd720_25frames.mjpeg", constants.RTCJPEGInitStatus, constants.RTCJPEGInitSuccess)
}

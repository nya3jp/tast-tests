// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     DecodeAccelVP9OddDimensions,
		Desc:     "Runs Chrome video_decode_accelerator_tests on a odd dimension VP9 video",
		Contacts: []string{"acourbot@chromium.org", "dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		// TODO(b/138915749): Enable once decoding odd dimension videos is fixed.
		Attr:         []string{"informational", "disabled"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP9},
		Data:         []string{"test-25fps-321x241.vp9", "test-25fps-321x241.vp9.json"},
	})
}

func DecodeAccelVP9OddDimensions(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoTestNew(ctx, s, "test-25fps-321x241.vp9", decode.VDA)
}

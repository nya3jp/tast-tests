// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCDecodeAccelVP8240P,
		Desc:         "Run arcvideodecoder_test on ARC++ with an 240p VP8 video test-25fps.vp8",
		Contacts:     []string{"johnylin@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login", caps.HWDecodeVP8},
		Data:         decode.DataFiles(videotype.VP8Prof, decode.ImportBuffer),
		Pre:          arc.Booted(),
	})
}

func ARCDecodeAccelVP8240P(ctx context.Context, s *testing.State) {
	decode.RunAllARCVideoTests(ctx, s, s.PreValue().(arc.PreData).ARC, decode.Test25FPSVP8)
}

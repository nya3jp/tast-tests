// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCDecodeAccelVP8240P,
		Desc:         "Runs arcvideodecoder_test on ARC++ with an 240p VP8 video test-25fps.vp8",
		Contacts:     []string{"johnylin@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome", caps.HWDecodeVP8},
		Data:         []string{"test-25fps.vp8", "test-25fps.vp8.json"},
		Pre:          arc.Booted(),
	})
}

func ARCDecodeAccelVP8240P(ctx context.Context, s *testing.State) {
	decode.RunAllARCVideoTests(ctx, s, "test-25fps.vp8")
}

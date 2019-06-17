// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCDecodeAccelPerfH264240P,
		Desc:         "Runs arcvideodecoder_test on ARC++ to measure the performance with an 240p H.264 video test-25fps.h264",
		Contacts:     []string{"johnylin@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"android", "chrome", caps.HWDecodeH264},
		Data:         []string{decode.Test25FPSH264.Name, decode.Test25FPSH264.Name + ".json"},
		Pre:          arc.Booted(),
	})
}

func ARCDecodeAccelPerfH264240P(ctx context.Context, s *testing.State) {
	decode.RunARCVideoPerfTest(ctx, s, s.PreValue().(arc.PreData).ARC, decode.Test25FPSH264.Name)
}

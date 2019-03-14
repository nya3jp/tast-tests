// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/libvda"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     LibvdaDecodeH264,
		Desc:     "Checks H.264 video decoding using libvda's Mojo connection to GAVDA is working",
		Contacts: []string{"alexlau@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"informational"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{"android", "chrome_internal", "chrome_login", caps.HWDecodeH264},
		Data:         []string{"test-25fps.h264"},
	})
}

func LibvdaDecodeH264(ctx context.Context, s *testing.State) {
	libvda.RunGpuFileDecodeTest(ctx, s, "output_libvda_h264.txt", "test-25fps.h264")
}

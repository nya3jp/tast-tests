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
		Func:         LibvdaDecodeVP9,
		Desc:         "Checks VP9 video decoding using libvda's Mojo connection to GAVDA is working",
		Contacts:     []string{"alexlau@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login", caps.HWDecodeVP9},
		Data:         []string{"test-25fps.vp9"},
	})
}

func LibvdaDecodeVP9(ctx context.Context, s *testing.State) {
	libvda.RunGpuFileDecodeTest(ctx, s, "output_libvda_vp9.txt", "test-25fps.vp9")
}

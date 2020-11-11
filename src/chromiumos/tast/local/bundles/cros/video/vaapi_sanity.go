// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
		Func: VaapiSanity,
		Desc: "Sanity checks libva decoding by running the media/gpu/vaapi/test:decode_test binary",
		Contacts: []string{
			"jchinlee@chromium.org",
			"chromeos-video-eng@google.com",
		},
		SoftwareDeps: []string{"vaapi"},
		Params: []testing.Param{{
			Name:              "vp9_profile0",
			Val:               "resolution_change_500frames.vp9.ivf",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"resolution_change_500frames.vp9.ivf", "resolution_change_500frames.vp9.ivf.json"},
		}, {
			Name:              "vp9_fail",
			Val:               "resolution_change_500frames.vp8.ivf",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"resolution_change_500frames.vp8.ivf", "resolution_change_500frames.vp8.ivf.json"},
		}},
	})
}

func VaapiSanity(ctx context.Context, s *testing.State) {
	decode.RunVaapiSanityTest(ctx, s, s.Param().(string))
}

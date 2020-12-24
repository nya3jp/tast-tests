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

type vaapiSmokeParams struct {
	filename     string
	expectedFail bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: VaapiSmoke,
		Desc: "Smoke tests libva decoding by running the media/gpu/vaapi/test:decode_test binary",
		Contacts: []string{
			"jchinlee@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"vaapi"},
		Params: []testing.Param{{
			Name:              "vp9_profile0",
			Val:               vaapiSmokeParams{filename: "resolution_change_500frames.vp9.ivf", expectedFail: false},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         []string{"resolution_change_500frames.vp9.ivf", "resolution_change_500frames.vp9.ivf.json"},
		}, {
			// Attempt to decode an unsupported codec to ensure that the binary is not
			// unconditionally succeeding, i.e. not crashing even when expected to.
			Name:      "unsupported_codec_fail",
			Val:       vaapiSmokeParams{filename: "resolution_change_500frames.vp8.ivf", expectedFail: true},
			ExtraData: []string{"resolution_change_500frames.vp8.ivf", "resolution_change_500frames.vp8.ivf.json"},
		}},
	})
}

func VaapiSmoke(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(vaapiSmokeParams)
	decode.RunVaapiSmokeTest(ctx, s, testOpt.filename, testOpt.expectedFail)
}

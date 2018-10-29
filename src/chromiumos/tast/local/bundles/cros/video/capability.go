// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"regexp"
	"strings"

	"chromiumos/tast/autocaps"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Capability,
		Desc: "Compare capabilities computed by autocaps package with ones detected avtest_label_detect",
		Attr: []string{"informational"},
	})
}

// avtestLabelToCapability is map from label detected by avtest_label_detect to capability computed by autocaps package.
// "[20:]" in values is to remove the prefix "autotest-capability:" from constant capability strings in caps package.
var avtestLabelToCapability = map[string]string{
	"hw_video_acc_h264":     caps.HWDecodeH264[20:],
	"hw_video_acc_vp8":      caps.HWDecodeVP8[20:],
	"hw_video_acc_vp9":      caps.HWDecodeVP9[20:],
	"hw_video_acc_vp9_2":    caps.HWDecodeVP9_2[20:],
	"hw_jpeg_acc_dec":       caps.HWDecodeJPEG[20:],
	"hw_video_acc_enc_h264": caps.HWEncodeH264[20:],
	"hw_video_acc_enc_vp8":  caps.HWEncodeVP8[20:],
	"hw_video_acc_enc_vp9":  caps.HWEncodeVP9[20:],
	"hw_jpeg_acc_enc":       caps.HWEncodeJPEG[20:],
	"webcam":                caps.USBCamera[20:],
}

// Capability compares the results between autocaps package and avtest_label_detect.
// The test failure is decided as follows, where OK and Fail stands for success and
// failure, respectively. For the capability marked "disable", we don't check
// them, because the capability is not disabled in driver level, but disabled in
// Chrome level by default, which an user can enable it by chrome://flags.
// avldetect \ autocaps |  Yes   |  No    | Disable  |
//       detect         |  OK    |  Fail  |   OK     |
//       not detect     |  Fail  |  OK    |   OK     |
func Capability(ctx context.Context, s *testing.State) {
	// Get capabilities computed by autocaps package.
	var info autocaps.SysInfo
	autoCaps, err := autocaps.Read("/usr/local/etc/autotest-capability", &info)
	if err != nil {
		s.Fatal("Failed to read caps in autotest package: ", err)
	}
	testing.ContextLog(ctx, "autocaps package result: ", autoCaps)

	// Get capabilities detected by "avtest_label_detect" command.
	cmd := testexec.CommandContext(ctx, "avtest_label_detect")
	avOut, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to execute avtest_label_detect: ", err)
	}
	testing.ContextLog(ctx, "avtest_label_detect result: ", avOut)

	var detectedLabelRegexp = regexp.MustCompile(`(?m)^Detected label: (.*)$`)
	avCaps := make(map[string]struct{})
	for _, m := range detectedLabelRegexp.FindAllStringSubmatch(string(avOut), -1) {
		label := strings.Trim(m[1], " ")
		if c, found := avtestLabelToCapability[label]; found {
			avCaps[c] = struct{}{}
		}
	}

	var mismatchCaps []string
	for _, c := range avtestLabelToCapability {
		_, avCapsHas := avCaps[c]

		switch autoCaps[c] {
		case autocaps.Yes:
			if !avCapsHas {
				s.Errorf("Static capability claims '%s' is available. But avtest_label_detect doesn't detect.", c)
				mismatchCaps = append(mismatchCaps, c)
			}
		case autocaps.No:
			if avCapsHas {
				s.Errorf("Static capability claims '%s' is not available. But avtest_label_detect detects.", c)
				mismatchCaps = append(mismatchCaps, c)
			}
		}
	}
}

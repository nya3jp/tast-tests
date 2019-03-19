// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"chromiumos/tast/autocaps"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Capability,
		Desc:     "Compare capabilities computed by autocaps package with ones detected by avtest_label_detect",
		Contacts: []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"informational"},
	})
}

// avtestLabelToCapability is map from label detected by avtest_label_detect to capability computed by autocaps package.
var avtestLabelToCapability = map[string]string{
	"hw_video_acc_h264":        caps.HWDecodeH264,
	"hw_video_acc_vp8":         caps.HWDecodeVP8,
	"hw_video_acc_vp9":         caps.HWDecodeVP9,
	"hw_video_acc_vp9_2":       caps.HWDecodeVP9_2,
	"hw_jpeg_acc_dec":          caps.HWDecodeJPEG,
	"hw_video_acc_enc_h264":    caps.HWEncodeH264,
	"hw_video_acc_enc_vp8":     caps.HWEncodeVP8,
	"hw_video_acc_enc_vp9":     caps.HWEncodeVP9,
	"hw_video_acc_enc_h264_4k": caps.HWEncodeH264_4K,
	"hw_video_acc_enc_vp8_4k":  caps.HWEncodeVP8_4K,
	"hw_video_acc_enc_vp9_4k":  caps.HWEncodeVP9_4K,
	"hw_jpeg_acc_enc":          caps.HWEncodeJPEG,
	"builtin_usb_camera":       caps.BuiltinUSBCamera,
	"builtin_mipi_camera":      caps.BuiltinMIPICamera,
	"vivid_camera":             caps.VividCamera,
	"builtin_camera":           caps.BuiltinCamera,
	"builtin_or_vivid_camera":  caps.BuiltinOrVividCamera,
}

// Capability compares the results between autocaps package and avtest_label_detect.
// The test failure is decided as follows, where OK and Fail stands for success and
// failure, respectively. For the capability marked "disable", we don't check
// them, because the capability is not disabled in driver level, but disabled in
// Chrome level by default, which an user can enable it by chrome://flags.
//  avldetect\autocaps | Yes  | No   | Disable |
//        detect       | OK   | Fail | OK      |
//        not detect   | Fail | OK   | OK      |
func Capability(ctx context.Context, s *testing.State) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	// Get capabilities computed by autocaps package.
	staticCaps, err := autocaps.Read(autocaps.DefaultCapabilityDir, nil)
	if err != nil {
		s.Fatal("Failed to read statically-set capabilities: ", err)
	}
	testing.ContextLog(ctx, "Statically-set capabilities: ", staticCaps)

	// Get capabilities detected by "avtest_label_detect" command.
	cmd := testexec.CommandContext(ctx, "avtest_label_detect")
	avOut, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to execute avtest_label_detect: ", err)
	}

	var detectedLabelRegexp = regexp.MustCompile(`(?m)^Detected label: (.*)$`)
	detectedCaps := make(map[string]struct{})
	for _, m := range detectedLabelRegexp.FindAllStringSubmatch(string(avOut), -1) {
		label := strings.TrimSpace(m[1])
		if c, found := avtestLabelToCapability[label]; found {
			detectedCaps[stripPrefix(c)] = struct{}{}
		}
	}
	testing.ContextLog(ctx, "avtest_label_detect result: ", detectedCaps)

	for _, c := range avtestLabelToCapability {
		c = stripPrefix(c)
		_, wasDetected := detectedCaps[c]
		state, ok := staticCaps[c]
		if !ok {
			s.Errorf("Static capabilities don't include %q", c)
			continue
		}

		switch state {
		case autocaps.Yes:
			if !wasDetected {
				s.Errorf("%q statically set but not detected", c)
			}
		case autocaps.No:
			if wasDetected {
				s.Errorf("%q detected but not statically set", c)
			}
		}
	}
}

// stripPrefix removes caps.Prefix from the beginning of cap.
func stripPrefix(cap string) string {
	if !strings.HasPrefix(cap, caps.Prefix) {
		panic(fmt.Sprintf("%q doesn't start with %q", cap, caps.Prefix))
	}
	return cap[len(caps.Prefix):]
}

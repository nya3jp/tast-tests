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
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Capability,
		Desc:     "Compare capabilities computed by autocaps package with ones detected by avtest_label_detect",
		Contacts: []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

// capability defines a single entry in the avtestLabelToCapability map.
type capability struct {
	name     string // The name of the capability
	optional bool   // Whether the capability is optional
}

// avtestLabelToCapability is a map from labels detected by avtest_label_detect to capabilities
// set in the autocaps package. Capabilities marked as optional can be omitted, even if they
// are detected by avtest_label_detect. This is e.g. necessary for devices that technically support
// 4K HW decoding, but don't have the autocaps labels set because these device are so slow that
// running 4K tests is a huge drain on lab resources.
// See /src/third_party/chromiumos-overlay/chromeos-base/autotest-capability-default/files/managed-capabilities.yaml
// for the meaning of each label.
var avtestLabelToCapability = map[string]capability{
	"hw_video_acc_h264":         {caps.HWDecodeH264, false},
	"hw_video_acc_vp8":          {caps.HWDecodeVP8, false},
	"hw_video_acc_vp9":          {caps.HWDecodeVP9, false},
	"hw_video_acc_vp9_2":        {caps.HWDecodeVP9_2, false},
	"hw_video_acc_av1":          {caps.HWDecodeAV1, false},
	"hw_video_acc_av1_10bpp":    {caps.HWDecodeAV1_10BPP, false},
	"hw_video_acc_h264_4k":      {caps.HWDecodeH264_4K, true},
	"hw_video_acc_vp8_4k":       {caps.HWDecodeVP8_4K, true},
	"hw_video_acc_vp9_4k":       {caps.HWDecodeVP9_4K, true},
	"hw_video_acc_av1_4k":       {caps.HWDecodeAV1_4K, true},
	"hw_video_acc_av1_4k_10bpp": {caps.HWDecodeAV1_4K10BPP, true},
	"hw_jpeg_acc_dec":           {caps.HWDecodeJPEG, false},
	"hw_video_acc_enc_h264":     {caps.HWEncodeH264, false},
	"hw_video_acc_enc_vp8":      {caps.HWEncodeVP8, false},
	"hw_video_acc_enc_vp9":      {caps.HWEncodeVP9, false},
	"hw_video_acc_enc_h264_4k":  {caps.HWEncodeH264_4K, false},
	"hw_video_acc_enc_vp8_4k":   {caps.HWEncodeVP8_4K, false},
	"hw_video_acc_enc_vp9_4k":   {caps.HWEncodeVP9_4K, false},
	"hw_jpeg_acc_enc":           {caps.HWEncodeJPEG, false},
	"builtin_usb_camera":        {caps.BuiltinUSBCamera, false},
	"builtin_mipi_camera":       {caps.BuiltinMIPICamera, false},
	"vivid_camera":              {caps.VividCamera, false},
	"builtin_camera":            {caps.BuiltinCamera, false},
	"builtin_or_vivid_camera":   {caps.BuiltinOrVividCamera, false},
}

// Capability compares the results between autocaps package and avtest_label_detect.
// The test failure is decided as follows, where OK and Fail stands for success and
// failure, respectively. For the capability marked "disable", we don't check
// them, because the capability is not disabled in driver level, but disabled in
// Chrome level by default, which an user can enable it by chrome://flags.
//  avldetect\autocaps        | Yes  | No   | Disable |
//        detected            | OK   | Fail | OK      |
//        detected (optional) | OK   | OK   | OK      |
//        not detected        | Fail | OK   | OK      |
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
			detectedCaps[stripPrefix(c.name)] = struct{}{}
		}
	}
	testing.ContextLog(ctx, "avtest_label_detect result: ", detectedCaps)

	for _, c := range avtestLabelToCapability {
		c.name = stripPrefix(c.name)
		_, wasDetected := detectedCaps[c.name]
		state, ok := staticCaps[c.name]
		if !ok {
			s.Errorf("Static capabilities don't include %q", c.name)
			continue
		}

		switch state {
		case autocaps.Yes:
			if !wasDetected {
				s.Errorf("%q statically set but not detected", c.name)
			}
		case autocaps.No:
			if wasDetected && !c.optional {
				s.Errorf("%q detected but not statically set and not optional", c.name)
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

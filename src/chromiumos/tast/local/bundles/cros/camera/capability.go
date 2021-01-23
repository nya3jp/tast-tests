// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Capability,
		Desc:     "Compare capabilities computed by autocaps package with ones detected by avtest_label_detect",
		Contacts: []string{"hiroh@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

// capabilitiesToVerify is a map of capabilities to verify indexed by the
// avtest_label_detect command line tool name.
var capabilitiesToVerify = map[string]caps.Capability{
	"builtin_usb_camera":      {caps.BuiltinUSBCamera, false},
	"builtin_mipi_camera":     {caps.BuiltinMIPICamera, false},
	"vivid_camera":            {caps.VividCamera, false},
	"builtin_camera":          {caps.BuiltinCamera, false},
	"builtin_or_vivid_camera": {caps.BuiltinOrVividCamera, false},
}

// Capability compares the static capabilities versus those detected in the DUT.
func Capability(ctx context.Context, s *testing.State) {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to set values for verbose logging")
	}
	defer vl.Close()

	if err := caps.VerifyCapabilities(ctx, capabilitiesToVerify); err != nil {
		s.Fatal("Test failed: ", err)
	}
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/autocaps"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Capability,
		Desc:     "Compare capabilities defined in autocaps package with ones detected by platform camera tools",
		Contacts: []string{"kamesan@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

// Capability compares the static capabilities versus those detected in the DUT.
func Capability(ctx context.Context, s *testing.State) {
	// Get capabilities defined in autocaps package.
	staticCaps, err := autocaps.Read(autocaps.DefaultCapabilityDir, nil)
	if err != nil {
		s.Fatal("Failed to read statically-set capabilities: ", err)
	}

	// Detect USB cameras.
	usbCams, err := testutil.USBCamerasFromV4L2Test(ctx)
	if err != nil {
		s.Fatal("Failed to get USB cameras: ", err)
	}
	hasUSB := len(usbCams) > 0

	// Detect MIPI cameras.
	mipiCams, err := testutil.GetMIPICamerasFromCrOSCameraTool(ctx)
	if err != nil {
		s.Fatal("Failed to get MIPI cameras: ", err)
	}
	hasMIPI := len(mipiCams) > 0

	hasVivid := testutil.IsVividDriverLoaded(ctx)

	capsToVerify := map[string]bool{
		"builtin_usb_camera":      hasUSB,
		"builtin_mipi_camera":     hasMIPI,
		"vivid_camera":            hasVivid,
		"builtin_camera":          hasUSB || hasMIPI,
		"builtin_or_vivid_camera": hasUSB || hasMIPI || hasVivid,
	}
	for c, detected := range capsToVerify {
		if staticCaps[c] == autocaps.Yes && !detected {
			s.Errorf("%q statically set but not detected", c)
		} else if staticCaps[c] != autocaps.Yes && detected {
			s.Errorf("%q detected but not statically set", c)
		}
	}
}

// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"bytes"
	"context"
	"regexp"
	"strings"

	"chromiumos/tast/local/bundles/cros/storage/util"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UsbHealth,
		Desc:         "Performs a SMART health check of usb drive",
		Contacts:     []string{"asavery@google.com"},
		SoftwareDeps: []string{"storage_wearout_detect"},
	})
}

// UsbHealth runs a SMART health test on a usb drive
func UsbHealth(ctx context.Context, s *testing.State) {
	usbDevice, err := util.RemovableDevice(ctx, false)
	if err != nil {
		s.Fatal("Failed to get removable device: ", err)
	}

	var out []byte
	out, err = util.RunSmartHealth(ctx, usbDevice)
	out = bytes.TrimSpace(out)
	lines := strings.Split(string(out), "\n")
	if err != nil {
		usbNoSupport := regexp.MustCompile(`.*Unknown USB bridge.*`)
		for _, line := range lines {
			match := usbNoSupport.FindStringSubmatch(line)
			if match != nil {
				s.Log("usb device does not support SMART")
				return
			}
		}
		s.Fatal("Failed to run smartctl -H: ", err)
	}

	status := util.ParseUsbHealth(ctx, lines)
	if status == util.Failing {
		s.Fatal("usb device failed smartctl health check")
	}
	if status == util.NotApplicable {
		s.Log("Test not applicable, SMART not suppported on device")
	}
}

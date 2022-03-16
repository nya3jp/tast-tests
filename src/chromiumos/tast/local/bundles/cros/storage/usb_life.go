// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"bytes"
	"context"
	"strings"

	"chromiumos/tast/common/storage"
	"chromiumos/tast/local/bundles/cros/storage/util"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UsbLife,
		Desc:         "Performs a SMART life used check of usb drive",
		Contacts:     []string{"asavery@google.com"},
		SoftwareDeps: []string{"storage_wearout_detect"},
	})
}

// UsbLife runs a SMART health test on a usb drive
func UsbLife(ctx context.Context, s *testing.State) {
	usbDevice, err := util.RemovableDevice(ctx, false)
	if err != nil {
		s.Fatal("Failed to get removable device: ", err)
	}

	var out []byte
	out, err = util.RunSmartInfo(ctx, usbDevice)
	out = bytes.TrimSpace(out)
	lines := strings.Split(string(out), "\n")

	var status storage.LifeStatus
	status, err = util.ParseUsbLife(ctx, lines)
	if status == util.Failing {
		s.Fatal("Device is almost EOL: ", err)
	}
	if status == util.NotApplicable {
		s.Log("Test not applicable, SMART not suppported on device")
	}
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BootedDeviceReporter,
		Desc:         "Verifies that the BootedDevice reporter identifies which device mode the DUT was booted from",
		Contacts:     []string{"cros-fw-engprod@google.com"},
		SoftwareDeps: []string{"crossystem"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func BootedDeviceReporter(ctx context.Context, s *testing.State) {
	r := reporters.New(s.DUT())

	bootType, err := r.BootedDevice(ctx)
	if err != nil {
		s.Fatal("Could not determine booted device type: ", err)
	}
	s.Log("Booted device type is: ", bootType)
}

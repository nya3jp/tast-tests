// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	upstartcommon "chromiumos/tast/common/upstart"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AdbOverUsb,
		Desc: "Checks that arc(vm)-adbd job is up and running when adb-over-usb feature available",
		Contacts: []string{
			"shuanghu@chromium.org",
			"tast-owners@google.com",
		},
		HardwareDeps: hwdep.D(
			// Available boards info, please refer to doc https://www.chromium.org/chromium-os/chrome-os-systems-supporting-adb-debugging-over-usb
			hwdep.Model("eve", "atlas", "nocturne", "straka"),
		),
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func AdbOverUsb(ctx context.Context, s *testing.State) {
	// For ARC-P
	if upstart.JobExists(ctx, "arc-adbd") {
		if err := upstart.WaitForJobStatus(ctx, "arc-adbd", upstartcommon.StartGoal, upstartcommon.RunningState, upstart.RejectWrongGoal, 30*time.Second); err != nil {
			s.Fatal("Failed to start arc(vm)-adbd: ", err)
		}
	}

	// For ARCVM
	if upstart.JobExists(ctx, "arcvm-adbd") {
		if err := upstart.WaitForJobStatus(ctx, "arcvm-adbd", upstartcommon.StartGoal, upstartcommon.RunningState, upstart.RejectWrongGoal, 30*time.Second); err != nil {
			s.Fatal("Failed to start arc(vm)-adbd: ", err)
		}
	}
}

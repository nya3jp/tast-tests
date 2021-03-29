// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PowerCameraPreviewPerf,
		Desc: "Measures the battery drain and camera statistics (e.g., dropped frames) during camera preview at 30/60 FPS",
		Contacts: []string{
			"springerm@chromium.org",
			"arcvm-eng@google.com",
		},
		Attr: []string{"group:mainline"},
		Params: []testing.Param{
			{
				Name:              "test_kukui_discharge",
				ExtraHardwareDeps: hwdep.D(hwdep.Platform("kukui")),
				Timeout:           15 * time.Minute,
			},
		},
	})
}

func PowerCameraPreviewPerf(ctx context.Context, s *testing.State) {
	testing.ContextLog(ctx, "ectool chargecontrol discharge")
	dischargeCmd := testexec.CommandContext(ctx,
		"ectool", "chargecontrol", "discharge")
	if err := dischargeCmd.Run(); err != nil {
		s.Error("Failed to run discharge: ", err)
	}

	testing.ContextLog(ctx, "sleep 1")
	if err := testing.Sleep(ctx, time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	testing.ContextLog(ctx, "ectool chargecontrol normal")
	chargeCmd := testexec.CommandContext(ctx,
		"ectool", "chargecontrol", "normal")
	if err := chargeCmd.Run(); err != nil {
		s.Error("Failed to run charge: ", err)
	}

	testing.ContextLog(ctx, "done")
}

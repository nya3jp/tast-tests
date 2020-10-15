// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package soundcardinit

import (
	"context"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Max98390FirstBoot,
		Desc:         "Verifies sound_card_init max98390 boot time calibration at the first boot time",
		Contacts:     []string{"judyhsiao@chromium.org", "cychiang@chromium.org"},
		SoftwareDeps: []string{"audio_play"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{
			{
				Name:              "nightfury",
				ExtraHardwareDeps: hwdep.D(hwdep.Model("nightfury")),
				Val: TestParameters{
					SoundCardID: "sofcmlmax98390d",
					AmpCount:    2,
				},
				Timeout: 7 * time.Minute,
			},
		},
	})
}

// Max98390FirstBoot Verifies sound_card_init works correctly at the first boot time.
func Max98390FirstBoot(ctx context.Context, s *testing.State) {

	ampCount := s.Param().(TestParameters).AmpCount

	if err := RemoveStopTimeFile(ctx); err != nil {
		s.Fatalf("Failed to rm %s: %v", StopTimeFile, err)
	}

	if err := RemoveBootTimeFile(ctx); err != nil {
		s.Fatalf("Failed to rm %s: %v", BootTimeFile, err)
	}

	if err := RemoveCalibFiles(ctx, ampCount); err != nil {
		s.Fatal("Failed to rm calib files: ", err)
	}

	// Run sound_card_init.
	runCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	if err := testexec.CommandContext(
		runCtx, "/sbin/initctl",
		"start", "sound_card_init",
		"SOUND_CARD_ID="+s.Param().(TestParameters).SoundCardID,
	).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run sound_card_init: ", err)
	}

	// Sleep for 1 second as initctl returns without waiting sound_card_init stops.
	testing.Sleep(ctx, duration)

	if err := VerifyUseVPD(ctx, ampCount); err != nil {
		s.Fatal("Failed to verify calib files using VPD value: ", err)
	}
}

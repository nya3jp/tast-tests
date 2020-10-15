// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package soundcardinit

import (
	"context"
	"fmt"
	"os"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Max98390,
		Desc:         "Verifies sound_card_init max98390 boot time calibration at the first boot time",
		Contacts:     []string{"judyhsiao@chromium.org", "cychiang@chromium.org"},
		SoftwareDeps: []string{"audio_play"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{
			{
				Name:              "first_boot",
				ExtraHardwareDeps: hwdep.D(hwdep.Model("nightfury")),
				Val: TestParameters{
					Func: firstBoot,
				},
				Timeout: 2 * time.Minute,
			},
			{
				Name:              "recent_reboot",
				ExtraHardwareDeps: hwdep.D(hwdep.Model("nightfury")),
				Val: TestParameters{
					Func: recentReboot,
				},
				Timeout: 2 * time.Minute,
			},
		},
	})
}

// TODO(b/171217019): parse sound_card_init yaml to get ampCount.
const ampCount = 2

// Max98390 Verifies sound_card_init  Max98390  boot time calibration logic.
func Max98390(ctx context.Context, s *testing.State) {
	soundCardID, err := GetSoundCardID(ctx)
	if err != nil {
		s.Fatal("Failed to get sound card name: ", err)
	}

	// Clear all sound_card_init files.
	if err := os.Remove(StopTimeFile); err != nil && !os.IsNotExist(err) {
		s.Fatalf("Failed to rm %s: %v", StopTimeFile, err)
	}

	bootTimeFile := fmt.Sprintf(BootTimeFile, soundCardID)
	if err := os.Remove(bootTimeFile); err != nil && !os.IsNotExist(err) {
		s.Fatalf("Failed to rm %s: %v", bootTimeFile, err)
	}

	if err := RemoveCalibFiles(ctx, soundCardID, ampCount); err != nil {
		s.Fatal("Failed to rm calib files: ", err)
	}

	// Run test cases.
	testFunc := s.Param().(TestParameters).Func
	testFunc(ctx, s, soundCardID)
}

// firstBoot Verifies sound_card_init works correctly at the first boot time.
func firstBoot(ctx context.Context, s *testing.State, soundCardID string) {
	// Run sound_card_init.
	runCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	if err := testexec.CommandContext(
		runCtx, "/sbin/initctl",
		"start", "sound_card_init",
		"SOUND_CARD_ID="+soundCardID,
	).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run sound_card_init: ", err)
	}

	// Sleep for 1 second as initctl returns without waiting for sound_card_init completion.
	testing.Sleep(ctx, duration)

	if err := VerifyUseVPD(ctx, soundCardID, ampCount); err != nil {
		s.Fatal("Failed to verify calib files using VPD value: ", err)
	}
}

// recentReboot Verifies sound_card_init max98390 skips boot time calibration after the recent reboot.
func recentReboot(ctx context.Context, s *testing.State, soundCardID string) {
	// Create previous boot time as yesterday.
	if err := CreateBootTimeFile(ctx, soundCardID, time.Now().AddDate(0, 0, -1).Unix()); err != nil {
		s.Fatalf("Failed to create %s: %v", BootTimeFile, err)
	}
	// Create previous CRAS stop time as now to mock the recent reboot.
	if err := CreateStopTimeFile(ctx, time.Now().Unix()); err != nil {
		s.Fatalf("Failed to create %s: %v", StopTimeFile, err)
	}

	// Run sound_card_init.
	runCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	if err := testexec.CommandContext(
		runCtx, "/sbin/initctl",
		"start", "sound_card_init",
		"SOUND_CARD_ID="+soundCardID,
	).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run sound_card_init: ", err)
	}

	// Sleep for 1 second as initctl returns without waiting for sound_card_init completion.
	testing.Sleep(ctx, duration)

	if err := VerifyCalibNotExist(ctx, soundCardID, ampCount); err != nil {
		s.Fatal("Boot time calibration should be skipped after the recent reboot: ", err)
	}
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"fmt"
	"os"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/audio/soundcardinit"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Max98390,
		Desc:         "Verifies sound_card_init max98390 boot time calibration at the first boot time",
		Contacts:     []string{"judyhsiao@chromium.org", "cychiang@chromium.org"},
		SoftwareDeps: []string{"audio_play"},
		HardwareDeps: hwdep.D(hwdep.Model("nightfury")),
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      1 * time.Minute,
		Params: []testing.Param{
			{
				Name: "first_boot",
				Val: soundcardinit.TestParameters{
					Func: firstBoot,
				},
			},
			{
				Name: "recent_reboot",
				Val: soundcardinit.TestParameters{
					Func: recentReboot,
				},
			},
		},
	})
}

// TODO(b/171217019): parse sound_card_init yaml to get ampCount.
const ampCount = 2
const timeout = 2 * time.Second

// Max98390 Verifies sound_card_init  Max98390  boot time calibration logic.
func Max98390(ctx context.Context, s *testing.State) {
	soundCardID, err := soundcardinit.GetSoundCardID(ctx)
	if err != nil {
		s.Fatal("Failed to get sound card name: ", err)
	}

	// Clear all sound_card_init files.
	if err := os.Remove(soundcardinit.StopTimeFile); err != nil && !os.IsNotExist(err) {
		s.Fatalf("Failed to rm %s: %v", soundcardinit.StopTimeFile, err)
	}

	runTimeFile := fmt.Sprintf(soundcardinit.RunTimeFile, soundCardID)
	if err := os.Remove(runTimeFile); err != nil && !os.IsNotExist(err) {
		s.Fatalf("Failed to rm %s: %v", runTimeFile, err)
	}

	if err := soundcardinit.RemoveCalibFiles(ctx, soundCardID, ampCount); err != nil {
		s.Fatal("Failed to rm calib files: ", err)
	}

	// Run test cases.
	testFunc := s.Param().(soundcardinit.TestParameters).Func
	testFunc(ctx, s, soundCardID)
}

// firstBoot Verifies sound_card_init works correctly at the first boot time.
func firstBoot(ctx context.Context, s *testing.State, soundCardID string) {
	// Run sound_card_init.
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := testexec.CommandContext(
		runCtx, "/sbin/initctl",
		"start", "sound_card_init",
		"SOUND_CARD_ID="+soundCardID,
	).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run sound_card_init: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return soundcardinit.VerifyUseVPD(ctx, soundCardID, ampCount)
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		s.Fatal("Failed to verify calib files using VPD value: ", err)
	}
}

// recentReboot Verifies sound_card_init max98390 skips boot time calibration after the recent reboot.
func recentReboot(ctx context.Context, s *testing.State, soundCardID string) {
	// Create previous sound_car_init run time as yesterday.
	if err := soundcardinit.CreateRunTimeFile(ctx, soundCardID, time.Now().AddDate(0, 0, -1).Unix()); err != nil {
		s.Fatal("Failed to create RunTimeFile: ", err)
	}
	// Create previous CRAS stop time as now to mock the recent reboot.
	if err := soundcardinit.CreateStopTimeFile(ctx, time.Now().Unix()); err != nil {
		s.Fatalf("Failed to create %s: %v", soundcardinit.StopTimeFile, err)
	}

	f := fmt.Sprintf(soundcardinit.RunTimeFile, soundCardID)
	startTime := time.Now()
	// Run sound_card_init.
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := testexec.CommandContext(
		runCtx, "/sbin/initctl",
		"start", "sound_card_init",
		"SOUND_CARD_ID="+soundCardID,
	).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run sound_card_init: ", err)
	}

	// Poll for sound_card_init run time file being updated, which means sound_card_init completes running.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		info, err := os.Stat(f)
		if err != nil {
			return errors.Wrapf(err, "failed to stat %s:", f)
		}
		if info.ModTime().After(startTime) {
			return nil
		}
		return errors.New(f + " is not updated")
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		s.Fatal("Failed to wait for sound_card_init completion: ", err)
	}

	// Verify calib files still not exist after sound_card_init completion.
	if err := soundcardinit.VerifyCalibNotExist(ctx, soundCardID, ampCount); err != nil {
		s.Fatal("Boot time calibration should be skipped after the recent reboot: ", err)
	}
}

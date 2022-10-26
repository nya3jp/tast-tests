// Copyright 2020 The ChromiumOS Authors
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
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/bundles/cros/audio/soundcardinit"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SoundCardInit,
		Desc: "Verifies sound_card_init boot time calibration logic",
		// b/178479311: Skip lindar and lillipup as they have un-calibrated smart amp so that we cannot run sound_card_init.
		// b/221241958: Skip helios as it is an old project before sound_card_init.
		HardwareDeps: hwdep.D(hwdep.SmartAmp(), hwdep.SkipOnModel("atlas", "nocturne", "volteer2", "lindar", "lillipup", "helios")),
		Contacts:     []string{"judyhsiao@chromium.org", "cychiang@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      1 * time.Minute,
		Params: []testing.Param{
			{
				Name:              "boot_time_calibration",
				ExtraHardwareDeps: hwdep.D(hwdep.SmartAmpBootTimeCalibration()),
				Val: soundcardinit.TestParameters{
					Func: bootTimeCalibration,
				},
			},
			{
				Name:              "recent_reboot",
				ExtraHardwareDeps: hwdep.D(hwdep.SmartAmpBootTimeCalibration()),
				Val: soundcardinit.TestParameters{
					Func: recentReboot,
				},
			},
		},
	})
}

// TODO(b/171217019): parse sound_card_init yaml to get ampCount.
const ampCount = 2
const initctlTimeout = 2 * time.Second
const soundCardInitTimeout = 10 * time.Second

// vpdFile is the file stores channel 0 RDC VPD value.
const vpdFile = "/sys/firmware/vpd/ro/dsm_calib_r0_0"

// SoundCardInit Verifies sound_card_init boot time calibration logic.
func SoundCardInit(ctx context.Context, s *testing.State) {

	if err := audio.WaitForDevice(ctx, audio.OutputStream); err != nil {
		s.Fatal("Failed to wait for output device: ", err)
	}

	soundCardID, err := soundcardinit.GetSoundCardID(ctx)
	if err != nil {
		s.Fatal("Failed to get sound card name: ", err)
	}

	if _, err := os.Stat(vpdFile); err != nil {
		s.Fatal("Failed to stat VPD file: ", err)
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
	s.Logf("Running test %q", s.TestName())
	if err := testFunc(ctx, soundCardID); err != nil {
		s.Fatalf("%s test failed: %v", s.TestName(), err)
	}
}

// bootTimeCalibration verifies sound_card_init boot time calibration works correctly.
func bootTimeCalibration(ctx context.Context, soundCardID string) error {

	// Run sound_card_init.
	runCtx, cancel := context.WithTimeout(ctx, soundCardInitTimeout)
	defer cancel()
	config, err := crosconfig.Get(ctx, "/audio/main", "sound-card-init-conf")
	if err != nil {
		return errors.Wrap(err, "cros_config /audio/main sound-card-init-conf failed")
	}
	amp, err := crosconfig.Get(ctx, "/audio/main", "speaker-amp")
	if err != nil {
		return errors.Wrap(err, "cros_config /audio/main speaker-amp failed")
	}

	if err := testexec.CommandContext(
		runCtx,
		"/usr/bin/sound_card_init",
		"--id="+soundCardID,
		"--conf="+config,
		"--amp="+amp,
	).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to run sound_card_init")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return soundcardinit.VerifyCalibExist(ctx, soundCardID, ampCount)
	}, &testing.PollOptions{Timeout: soundCardInitTimeout}); err != nil {
		return errors.Wrap(err, "failed to verify calib files exists")
	}

	return nil
}

// recentReboot Verifies sound_card_init skips boot time calibration after the recent reboot.
func recentReboot(ctx context.Context, soundCardID string) error {
	// Create previous sound_car_init run time as yesterday.
	if err := soundcardinit.CreateRunTimeFile(ctx, soundCardID, time.Now().AddDate(0, 0, -1).Unix()); err != nil {
		return errors.Wrap(err, "failed to create RunTimeFile")
	}
	// Create previous CRAS stop time as now to mock the recent reboot.
	if err := soundcardinit.CreateStopTimeFile(ctx, time.Now().Unix()); err != nil {
		return errors.Wrapf(err, "failed to create %s", soundcardinit.StopTimeFile)
	}

	// Run sound_card_init.
	runCtx, cancel := context.WithTimeout(ctx, initctlTimeout)
	defer cancel()
	if err := testexec.CommandContext(
		runCtx, "/sbin/initctl",
		"start", "sound_card_init",
		"SOUND_CARD_ID="+soundCardID,
	).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to run sound_card_init")
	}
	//Wait for sound_card_init completion.
	testing.Sleep(ctx, soundCardInitTimeout)

	// Verify calib files still do not exist after sound_card_init completion.
	if err := soundcardinit.VerifyCalibNotExist(ctx, soundCardID, ampCount); err != nil {
		return errors.Wrap(err, "boot time calibration should be skipped after the recent reboot")
	}

	return nil
}

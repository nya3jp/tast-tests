// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/fingerprint/rpcdut"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SoundCardInit,
		Desc: "Verifies sound_card_init finishes successfully at boot time",
		// Skips atlas, nocturne, lindar, lillipup  as they don't use sound_card_init to initialized their smart amps.
		// Skip volteer2 as it's a reference design device not an official launched device.
		HardwareDeps: hwdep.D(hwdep.SmartAmp(), hwdep.SkipOnModel("atlas", "nocturne", "volteer2", "lindar", "lillipup")),
		Contacts:     []string{"judyhsiao@chromium.org", "yuhsuan@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      5 * time.Minute,
	})
}

const (
	soundCardInitTimeout = time.Minute

	// TODO(b/171217019): parse sound_card_init yaml to get ampCount.
	ampCount = 2

	// soundCardInitRunTimeFile is the file stores previous sound_card_init run time.
	soundCardInitRunTimeFile = "/var/lib/sound_card_init/%s/run"

	// crasStopTimeFile is the file stores previous CRAS stop time.
	crasStopTimeFile = "/var/lib/cras/stop"

	// calibrationFiles is the file stores previous calibration values.
	calibrationFiles = "/var/lib/sound_card_init/%s/calib_%d"

	// calibYAMLContent is the content of calibration file.
	calibYAMLContent = `
---
UseVPD
`
)

func parseSoundCardID(dump string) (string, error) {
	re := regexp.MustCompile(`card 0: ([a-z0-9]+) `)
	m := re.FindStringSubmatch(dump)

	if len(m) != 2 {
		return "", errors.New("no sound card")
	}
	return m[1], nil
}

// removeSoundCardInitFiles removes all sound_card_init files.
func removeSoundCardInitFiles(ctx context.Context, d *rpcdut.RPCDUT, soundCardID string) error {
	fs := dutfs.NewClient(d.RPC().Conn)
	if err := fs.Remove(ctx, crasStopTimeFile); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to rm file: %s", crasStopTimeFile)
	}
	file := fmt.Sprintf(soundCardInitRunTimeFile, soundCardID)
	if err := fs.Remove(ctx, file); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to rm file: %s", file)
	}
	return nil
}

// createCalibrationFiles creates the calibration files on DUT.
func createCalibrationFiles(ctx context.Context, d *rpcdut.RPCDUT, soundCardID string, count uint) error {
	fs := dutfs.NewClient(d.RPC().Conn)
	for i := 0; i < int(count); i++ {
		f := fmt.Sprintf(calibrationFiles, soundCardID, i)
		exists, err := fs.Exists(ctx, f)
		if err != nil {
			return errors.Wrapf(err, "failed to stat %s", f)
		}
		if !exists {
			if err := fs.WriteFile(ctx, f, []byte(calibYAMLContent), 0644); err != nil {
				return errors.Wrapf(err, "failed to create %s", f)
			}
		}
	}
	return nil
}

// verifySoundCardInitFinished polls for sound_card_init run time file being updated, which means sound_card_init completes running.
func verifySoundCardInitFinished(ctx context.Context, d *rpcdut.RPCDUT, soundCardID string) error {
	fs := dutfs.NewClient(d.RPC().Conn)
	err := testing.Poll(ctx, func(ctx context.Context) error {
		file := fmt.Sprintf(soundCardInitRunTimeFile, soundCardID)
		exists, err := fs.Exists(ctx, file)
		if err != nil {
			return errors.Wrapf(err, "failed to stat %s", file)
		}
		if exists {
			return nil
		}
		return errors.New(file + " does not exist")
	}, &testing.PollOptions{Timeout: soundCardInitTimeout})
	return err
}

// SoundCardInit verifies sound_card_init finishes successfully at boot time.
func SoundCardInit(ctx context.Context, s *testing.State) {
	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	d, err := rpcdut.NewRPCDUT(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect RPCDUT: ", err)
	}
	// Ensure the rpc connection is closed at the end of this test.
	defer func(ctx context.Context) {
		if err := d.Close(ctx); err != nil {
			s.Fatal("Failed to close RPCDUT: ", err)
		}
	}(cleanupCtx)

	dump, err := d.Conn().CommandContext(ctx, "aplay", "-l").Output()
	if err != nil {
		s.Fatal("Failed to aplay -l: ", err)
	}
	soundCardID, err := parseSoundCardID(string(dump))
	if err != nil {
		s.Fatal("Failed to parse sound card name: ", err)
	}

	if err := removeSoundCardInitFiles(ctx, d, soundCardID); err != nil {
		s.Fatal("Failed to remove previous files: ", err)
	}

	if err := createCalibrationFiles(ctx, d, soundCardID, ampCount); err != nil {
		s.Fatal("Failed to create calibration files: ", err)
	}

	s.Log("Reboot the device")
	// Reboot
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot the DUT: ", err)
	}

	// Poll for sound_card_init run time file being updated, which means sound_card_init completes running.
	if err := verifySoundCardInitFinished(ctx, d, soundCardID); err != nil {
		s.Fatal("Failed to wait for sound_card_init completion: ", err)
	}
}

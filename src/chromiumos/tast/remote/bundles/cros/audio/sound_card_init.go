// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/fingerprint/rpcdut"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SoundCardInit,
		Desc:         "Verifies sound_card_init finish successfully at boot time",
		HardwareDeps: hwdep.D(hwdep.SmartAmp(), hwdep.SkipOnModel("atlas", "nocturne", "volteer2")),
		Contacts:     []string{"judyhsiao@chromium.org", "yuhsuan@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      1 * time.Minute,
	})
}

const soundCardInitTimeout = 10 * time.Second

// runTimeFile is the file stores previous sound_card_init run time.
const runTimeFile = "/var/lib/sound_card_init/%s/run"

// StopTimeFile is the file stores previous CRAS stop time.
const stopTimeFile = "/var/lib/cras/stop"

func parseSoundCardID(dump string) (string, error) {
	re := regexp.MustCompile(`card 0: [a-z,0-9]+ `)
	str := re.FindString(string(dump))
	if str == "" {
		return "", errors.New("no sound card")
	}
	return strings.Trim(strings.TrimLeft(str, "card 0: "), " "), nil
}

func removeSoundCardInitFiles(ctx context.Context, d *rpcdut.RPCDUT, soundCardID string) error {
	// Clear all sound_card_init files.
	fs := dutfs.NewClient(d.RPC().Conn)
	if err := fs.Remove(ctx, stopTimeFile); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "%s: %v", stopTimeFile, err)
	}
	file := fmt.Sprintf(runTimeFile, soundCardID)
	if err := fs.Remove(ctx, file); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "%s: %v", file, err)
	}
	return nil
}

func verifySoundCardInitFinished(ctx context.Context, d *rpcdut.RPCDUT, soundCardID string) error {
	// Poll for sound_card_init run time file being updated, which means sound_card_init completes running.
	fs := dutfs.NewClient(d.RPC().Conn)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		file := fmt.Sprintf(runTimeFile, soundCardID)
		exists, err := fs.Exists(ctx, file)
		if err != nil {
			return errors.Wrapf(err, "failed to stat %s", file)
		}
		if exists {
			return nil
		}
		return errors.New(file + " does not exist")
	}, &testing.PollOptions{Timeout: soundCardInitTimeout}); err != nil {
		return err
	}
	return nil
}

// SoundCardInit verifies sound_card_init finish successfully at boot time.
func SoundCardInit(ctx context.Context, s *testing.State) {

	d, err := rpcdut.NewRPCDUT(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect RPCDUT: ", err)
	}
	defer func(ctx context.Context) {
		if err := d.Close(ctx); err != nil {
			s.Fatal("Failed to close RPCDUT: ", err)
		}
	}(ctx)

	dump, err := d.Conn().CommandContext(ctx, "aplay", "-l").Output()
	if err != nil {
		s.Fatal("Failed to aplay -l: ", err)
	}
	soundCardID, err := parseSoundCardID(string(dump))
	if err != nil {
		s.Fatal("Failed to parse sound card name: ", err)
	}

	if err := removeSoundCardInitFiles(ctx, d, soundCardID); err != nil {
		s.Fatal("Failed to rm file: ", err)
	}

	// Reboot
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot dut: ", err)
	}

	// Poll for sound_card_init run time file being updated, which means sound_card_init completes running.
	if err := verifySoundCardInitFinished(ctx, d, soundCardID); err != nil {
		s.Fatal("Failed to wait for sound_card_init completion: ", err)
	}
}

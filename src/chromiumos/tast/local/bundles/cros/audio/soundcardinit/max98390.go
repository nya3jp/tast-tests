// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package soundcardinit

import (
	"context"
	"io/ioutil"
	"strings"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const stopTimeFile = "/var/lib/cras/stop"
const bootTimeFile = "/var/lib/sound_card_init/sofcmlmax98390d/boot"
const calib0 = "/var/lib/sound_card_init/sofcmlmax98390d/calib_0"
const calib1 = "/var/lib/sound_card_init/sofcmlmax98390d/calib_1"
const duration = 1 * time.Second

type testParameters struct {
	SoundCardID string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Max98390,
		Desc:         "Verifies sound_card_init function works correctly for max98390",
		Contacts:     []string{"judyhsiao@chromium.org", "cychiang@chromium.org"},
		SoftwareDeps: []string{"audio_play"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{
			{
				Name:              "nightfury",
				ExtraHardwareDeps: hwdep.D(hwdep.Model("nightfury")),
				Val: testParameters{
					SoundCardID: "sofcmlmax98390d",
				},
				Timeout: 7 * time.Minute,
			},
		},
	})
}

// Max98390 Verifies sound_card_init works correctly at the first boot time.
func Max98390(ctx context.Context, s *testing.State) {
	s.Log("Run testFirstBoot")
	testFirstBoot(ctx, s)

	runCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	// Restart CRAS to which means a reboot for sound_card_init.
	if err := testexec.CommandContext(
		runCtx, "stop", "cras",
	).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to stop cras")
	}
	if err := testexec.CommandContext(
		runCtx, "start", "cras",
	).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to start cras")
	}

	s.Log("Run testFastReboot")
	testFastReboot(ctx, s)
}

func testFirstBoot(ctx context.Context, s *testing.State) {
	runCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	// Remove stopTimeFile, bootTimeFile, calib*
	if err := testexec.CommandContext(
		runCtx, "rm", "-f",
		stopTimeFile,
	).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to rm %s: %v", stopTimeFile, err)
	}
	if err := testexec.CommandContext(
		runCtx, "rm", "-f",
		bootTimeFile,
	).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to rm %s: %v", bootTimeFile, err)
	}
	if err := testexec.CommandContext(
		runCtx, "rm", "-f",
		calib0,
	).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to rm %s: %v", calib0, err)
	}
	if err := testexec.CommandContext(
		runCtx, "rm", "-f",
		calib1,
	).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to rm %s: %v", calib1, err)
	}

	// Run sound_card_init.
	if err := testexec.CommandContext(
		runCtx, "/sbin/initctl",
		"start", "sound_card_init",
		"SOUND_CARD_ID="+s.Param().(testParameters).SoundCardID,
	).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run sound_card_init: ", err)
	}
	// Sleep for 1 second as initctl returns without waiting sound_card_init stops.
	testing.Sleep(ctx, duration)

	// Verify calib* contents.
	b, err := ioutil.ReadFile(calib0)
	if err != nil {
		s.Fatalf("Failed to read %s: %v", calib0, err)
	}
	if !strings.Contains(string(b), "UseVPD") {
		s.Fatalf("%s expect:UseVPD, got: %s", calib0, string(b))
	}

	b, err = ioutil.ReadFile(calib1)
	if err != nil {
		s.Fatalf("Failed to read %s: %v", calib1, err)
	}
	if !strings.Contains(string(b), "UseVPD") {
		s.Fatalf("%s expect:UseVPD, got: %s", calib1, string(b))
	}
}

func testFastReboot(ctx context.Context, s *testing.State) {
	runCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	// Run sound_card_init.
	if err := testexec.CommandContext(
		runCtx, "/sbin/initctl",
		"start", "sound_card_init",
		"SOUND_CARD_ID="+s.Param().(testParameters).SoundCardID,
	).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run sound_card_init: ", err)
	}
	// Sleep for 1 second as initctl returns without waiting sound_card_init stops.
	testing.Sleep(ctx, duration)

	// Verify calib* contents.
	b, err := ioutil.ReadFile(calib0)
	if err != nil {
		s.Fatalf("Failed to read %s: %v", calib0, err)
	}
	if !strings.Contains(string(b), "UseVPD") {
		s.Fatalf("%s expect:UseVPD, got: %s", calib0, string(b))
	}

	b, err = ioutil.ReadFile(calib1)
	if err != nil {
		s.Fatalf("Failed to read %s: %v", calib1, err)
	}
	if !strings.Contains(string(b), "UseVPD") {
		s.Fatalf("%s expect:UseVPD, got: %s", calib1, string(b))
	}
}

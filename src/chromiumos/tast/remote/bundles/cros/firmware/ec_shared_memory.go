// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECSharedMemory,
		Desc:         "Checks that there is still EC shared memory available",
		Contacts:     []string{"pf@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable", "firmware_bringup"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Fixture:      fixture.NormalMode,
	})
}

func ECSharedMemory(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.Servo.RunECCommand(ctx, "chan 0"); err != nil {
		s.Fatal("Failed to reset channel: ", err)
	}

	defer func() {
		if err := h.Servo.RunECCommand(ctx, "chan 0xffffffff"); err != nil {
			s.Fatal("Failed to reset channel: ", err)
		}
	}()

	s.Log("Check shared memory in normal operation")
	if err := checkSharedMemory(ctx, h); err != nil {
		s.Fatal("Failed to check shared memory: ", err)
	}

	s.Log("Crash EC unaligned")
	if err := h.Servo.RunECCommand(ctx, "crash unaligned"); err != nil {
		s.Fatal("Failed to send 'crash unaligned' to EC: ", err)
	}

	if err := h.DUT.WaitConnect(ctx); err != nil {
		s.Fatal("Failed connect to DUT: ", err)
	}

	s.Log("Check shared memory after crash")
	if err := checkSharedMemory(ctx, h); err != nil {
		s.Fatal("Failed to check shared memory after crash: ", err)
	}

	activeCopy, err := h.Servo.GetString(ctx, "ec_active_copy")
	if err != nil {
		s.Fatal("EC active copy failed: ", err)
	}
	if !strings.HasPrefix(activeCopy, "RW") {
		s.Logf("EC active copy got %q want RW, perform sysjump", activeCopy)
		if err := h.Servo.RunECCommand(ctx, "sysjump RW"); err != nil {
			s.Fatal("Failed to send 'sysjump RW' to EC: ", err)
		}
		activeCopy, err = h.Servo.GetString(ctx, "ec_active_copy")
		if err != nil {
			s.Fatal("EC active copy failed: ", err)
		}
		if !strings.HasPrefix(activeCopy, "RW") {
			s.Fatalf("Expected EC to be in RW, but was %q", activeCopy)
		}
	}

	s.Log("Check shared memory after sysjump")
	if err := checkSharedMemory(ctx, h); err != nil {
		s.Fatal("Failed to check shared memory after sysjump: ", err)
	}

	s.Log("Rebooting EC to restore default state")
	if err := h.Servo.RunECCommand(ctx, "reboot"); err != nil {
		s.Fatal("Failed to reboot EC: ", err)
	}
}

func checkSharedMemory(ctx context.Context, h *firmware.Helper) error {
	const (
		// EC Shared Memory level which lead to warning
		warningLevel = 256
		// EC Shared Memory level which lead to error
		errorLevel = 0
	)

	ecShmemOut, err := h.Servo.RunECCommandGetOutput(ctx, "shmem", []string{`Size:\s*([0-9-]+)`})
	if err != nil {
		return errors.Wrap(err, "failed to read EC shared memory size")
	}

	testing.ContextLogf(ctx, "EC shared memory size is %s bytes", ecShmemOut[0][1])
	ecShmemStr := ecShmemOut[0][1]
	ecShmem, err := strconv.ParseInt(ecShmemStr, 10, 64)
	if err != nil {
		return errors.Wrapf(err, "failed to parse EC shared memory (%s) as int",
			ecShmemStr)
	}

	if ecShmem <= errorLevel {
		return errors.Errorf("EC shared memory size is too small")
	} else if ecShmem <= warningLevel {
		testing.ContextLogf(ctx, "EC shared memory is less than %d bytes", warningLevel)
	}

	return nil
}

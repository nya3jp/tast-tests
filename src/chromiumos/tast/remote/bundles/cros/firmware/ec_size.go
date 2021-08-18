// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strconv"

	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECSize,
		Desc:         "Compare ec flash size to expected ec size from a chip-to-size map",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Data:         []string{firmware.ConfigFile},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		Pre:          pre.NormalMode(),
		Vars:         []string{"servo"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

var chipSizeMap = map[string]int{
	"npcx_uut": 512, // (512 * 1024) bytes
}

func ECSize(ctx context.Context, s *testing.State) {
	h := s.PreValue().(*pre.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	sizeStr, err := h.Servo.GetString(ctx, "ec_flash_size")
	if err != nil {
		s.Fatal("Failed get output: ", err)
	}
	size, err := strconv.Atoi(sizeStr)
	if err != nil {
		s.Fatalf("Failed to convert size string %v to int", sizeStr)
	}

	chip, err := h.Servo.GetString(ctx, "ec_chip")
	if err != nil {
		s.Fatal("Failed get output: ", err)
	}

	s.Logf("Flash size: %v KB", size)
	s.Log("EC Chip: ", chip)

	expSize, ok := chipSizeMap[chip]
	if !ok {
		s.Fatalf("Chip %v not found in chipSizeMap", chip)
	}
	if expSize != size {
		s.Errorf("Expected EC size %d KB, got EC size %d KB for chip %v", expSize, size, chip)
	}
}

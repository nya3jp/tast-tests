// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECSize,
		Desc:         "Compare ec flash size to expected ec size from a chip-to-size map",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_ec"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Fixture:      fixture.NormalMode,
	})
}

// in alphabetical order
var chipSizeMap = map[string]int{
	"it83xx":          512, // (512 * 1024) bytes
	"ite_spi_ccd_i2c": 1024,
	"it8xxx2":         1024,
	"mec1322":         256,
	"npcx_int_spi":    512,
	"npcx_spi":        512,
	"npcx_uut":        512,
	"stm32":           256,
}

func ECSize(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	sizeInBytes, err := firmware.NewECTool(h.DUT, firmware.ECToolNameMain).FlashSize(ctx)
	if err != nil {
		s.Fatal("Failed to get flashinfo from ectool: ", err)
	}
	size := sizeInBytes / 1024

	chip, err := h.Servo.GetECChip(ctx)
	if err != nil {
		s.Fatal("Failed to get ec chip: ", err)
	}

	s.Logf("Flash size: %d KB", size)
	s.Log("EC Chip: ", chip)

	expSize, ok := chipSizeMap[chip]
	if !ok {
		s.Fatalf("Failed to find ec chip %v in chipSizeMap", chip)
	}
	if expSize != size {
		s.Fatalf("Failed to verify EC size, want %d KB, got %d KB for chip %v", expSize, size, chip)
	}
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/testing"
	"chromiumos/tast/common/firmware/ti50"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WorkstationTi50Demo,
		Desc: "Demo ti50 in workstation environment(Andreiboard connected to USB)",
		Timeout:      8 * time.Minute,
		Vars:         []string{"image", "spiflash"},
		Contacts: []string{
			"aluo@chromium.org",       // Test Author
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		Attr:         []string{"group:mainline"},
	})
}

func WorkstationTi50Demo(ctx context.Context, s *testing.State) {

    image, _ := s.Var("image")
    spiflash, _ := s.Var("spiflash")

    if spiflash == "" {
        spiflash = "/mnt/host/source/src/platform/cr50-utils/software/tools/SPI/spiflash"
    }

    s.Logf("Using image: %v", image)
    s.Logf("Using spiflash binary: %v", spiflash)

    board, err := ti50.CreateConnectedDemoBoard(ctx, spiflash)
    if err != nil {
    	s.Fatal("Could not connect to board", err)
    }
    if err := ti50.Demo(ctx, board, image, spiflash); err != nil {
        s.Fatalf("Demo Failed: %v", err)
    }
}

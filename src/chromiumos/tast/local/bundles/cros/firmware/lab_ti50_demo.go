// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/testing"
	"chromiumos/tast/common/firmware/ti50"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LabTi50Demo,
		Desc: "Demo ti50 in lab environment (connected to fizz box)",
		Contacts: []string{
			"aluo@chromium.org",       // Test Author
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		Attr:         []string{"group:mainline"},
	})
}

func LabTi50Demo(ctx context.Context, s *testing.State) {

        //TODO: Download and install UD Drivers from gs bucket.
        //TODO: Unzip ti50.tar.bz2 and flash the board.
    board, err := ti50.CreateConnectedDemoBoard(ctx, "/tmp/spiflash")
    if err != nil {
        s.Fatal("Could not connect to board", err)
    }
    if err := ti50.Demo(ctx, board, "/tmp/full_image.signed", "/tmp/spiflash"); err != nil {
        s.Fatalf("Demo Failed: %v", err)
    }
}

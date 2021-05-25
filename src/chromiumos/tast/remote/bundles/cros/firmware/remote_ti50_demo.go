// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/rpc"
	"chromiumos/tast/testing"
	"chromiumos/tast/common/firmware/ti50"
	remoteTi50 "chromiumos/tast/remote/firmware/ti50"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RemoteTi50Demo,
		Desc: "Demo ti50 in remote environment(Andreiboard connected to labstation)",
		Timeout:      5 * time.Second,
		Vars:         []string{"image", "spiflash"},
		Contacts: []string{
			"aluo@chromium.org",       // Test Author
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		ServiceDeps: []string{"tast.cros.baserpc.FileSystem", "tast.cros.firmware.SerialPortService"},
		Attr:         []string{"group:mainline"},
	})
}

func RemoteTi50Demo(ctx context.Context, s *testing.State) {

    image, _ := s.Var("image")
    spiflash, _ := s.Var("spiflash")

    if spiflash == "" {
        spiflash = "/mnt/host/source/src/platform/cr50-utils/software/tools/SPI/spiflash"
    }

    s.Logf("Using image: %v", image)
    s.Logf("Using spiflash binary: %v", spiflash)

    targets, err := remoteTi50.ListRemoteUltraDebugTargets(ctx, s.DUT())
        if err != nil {
		s.Fatal("Error finding UD targets")
        } else if len(targets) == 0 {
		s.Fatal("No UD targets found on device")
        } else {
                s.Logf("UD Targets: %v", targets)
        }

        tty := string(targets[0])

    rpcClient, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
    defer rpcClient.Close(ctx)

    if err != nil {
    	s.Fatal("rpcDial: ", err)
    }
        board := remoteTi50.NewRemoteAndreiBoard(s.DUT(), rpcClient.Conn, tty, 4096, spiflash, time.Microsecond*1)

    if err := ti50.Demo(ctx, board, image, spiflash); err != nil {
        s.Fatalf("Demo Failed: %v", err)
    }
}

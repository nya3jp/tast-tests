// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/common/firmware/ti50"
	remoteTi50 "chromiumos/tast/remote/firmware/ti50"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:    Ti50Demo,
		Desc:    "Demo ti50 in remote environment(Andreiboard connected to labstation)",
		Timeout: 1 * time.Minute,
		Vars:    []string{"image", "spiflash", "mode"},
		Contacts: []string{
			"aluo@chromium.org",            // Test Author
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		ServiceDeps: []string{"tast.cros.baserpc.FileSystem", "tast.cros.firmware.SerialPortService"},
		Attr:        []string{"group:firmware"},
	})
}

func Ti50Demo(ctx context.Context, s *testing.State) {

	mode, _ := s.Var("mode")
	spiflash, _ := s.Var("spiflash")

	board, rpcClient, err := remoteTi50.GetTi50TestBoard(ctx, s.DUT(), s.RPCHint(), mode, spiflash, 4096, 100*time.Millisecond)

	if err != nil {
		s.Fatal("Could not start Ti50Demo: ", err)
	}
	if rpcClient != nil {
		defer rpcClient.Close(ctx)
	}
	defer board.Close(ctx)

	image, _ := s.Var("image")
	s.Log("Using image at: ", image)

	if err = ti50.Demo(ctx, board, image); err != nil {
		s.Fatal("Ti50Demo Failed: ", err)
	}
}

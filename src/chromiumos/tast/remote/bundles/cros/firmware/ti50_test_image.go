// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/remote/firmware/ti50"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:    Ti50TestImage,
		Desc:    "Tast wrapper to run ti50 test projects",
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

func Ti50TestImage(ctx context.Context, s *testing.State) {
	successRe := regexp.MustCompile(`\S* App Test: SUCCESS`)

	mode, _ := s.Var("mode")
	spiflash, _ := s.Var("spiflash")

	board, rpcClient, err := ti50.GetTi50TestBoard(ctx, s.DUT(), s.RPCHint(), mode, spiflash, 16384, 1*time.Second)

	if err != nil {
		s.Fatal("Could not start Ti50TestImage: ", err)
	}
	if rpcClient != nil {
		defer rpcClient.Close(ctx)
	}
	defer board.Close(ctx)

	image, _ := s.Var("image")
	s.Log("Using image at: ", image)

	if image == "" {
		err = board.Reset(ctx)
	} else {
		err = board.FlashImage(ctx, image)
	}
	if err != nil {
		s.Fatal("Could not reset board: ", err)
	}
	captures, err := board.ReadSerialSubmatch(ctx, successRe)
	if err != nil {
		s.Fatalf("Could not detect success expression %q: %v", successRe, err)
	}
	s.Logf("Captured success message: %q", captures[0])
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/common/firmware/ti50"
	"chromiumos/tast/remote/firmware/ti50/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:    Ti50Demo,
		Desc:    "Demo ti50 in remote environment(Andreiboard connected to labstation)",
		Timeout: 1 * time.Minute,
		Vars:    []string{"image"},
		Contacts: []string{
			"aluo@chromium.org",            // Test Author
			"chromeos-firmware@google.com", // CrOS Firmware Developers
		},
		Data:        []string{"ti50_ti50_Unknown_PrePVT_ti50-accessory-nodelocked-ro-premp.bin"},
		ServiceDeps: []string{"tast.cros.baserpc.FileSystem", "tast.cros.firmware.SerialPortService"},
		Attr:        []string{"group:firmware"},
		Fixture:     fixture.DevBoardService,
	})
}

func Ti50Demo(ctx context.Context, s *testing.State) {

	f := s.FixtValue().(*fixture.Value)

	board, err := f.DevBoard(ctx, 4096, time.Second)
	if err != nil {
		s.Fatal("Could not get board: ", err)
	}

	image, ok := s.Var("image")

	if ok {
		if image != "" {
			s.Log("Overriding build image with that from cmdline at: ", image)
		} else {
			s.Log("Empty image provided on cmdline, skip flash")
		}
	} else {
		image = s.DataPath("ti50_ti50_Unknown_PrePVT_ti50-accessory-nodelocked-ro-premp.bin")
		s.Log("Using image from build at: ", image)
	}

	if err = ti50.Demo(ctx, board, image); err != nil {
		s.Fatal("Ti50Demo Failed: ", err)
	}
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/firmware/ti50"
	"chromiumos/tast/common/testexec"
	remoteTi50 "chromiumos/tast/remote/firmware/ti50"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:    Ti50CcdUpdate,
		Desc:    "Ti50 firmware update over CCD using gsctool",
		Timeout: 5 * time.Minute,
		Vars:    []string{"mode", "spiflash", "heximage", "binimage", "serial"},
		Contacts: []string{
			"ecgh@chromium.org",
			"ti50-core@google.com",
		},
		ServiceDeps: []string{"tast.cros.baserpc.FileSystem", "tast.cros.firmware.SerialPortService"},
		Attr:        []string{"group:firmware"},
	})
}

func Ti50CcdUpdate(ctx context.Context, s *testing.State) {
	const Ti50USBID = "18d1:504a"

	mode, _ := s.Var("mode")
	spiflash, _ := s.Var("spiflash")

	board, rpcClient, err := remoteTi50.GetTi50TestBoard(ctx, s.DUT(), s.RPCHint(), mode, spiflash, 4096, 100*time.Millisecond)

	if err != nil {
		s.Fatal("GetTi50TestBoard: ", err)
	}
	if rpcClient != nil {
		defer rpcClient.Close(ctx)
	}
	defer board.Close(ctx)

	heximage, _ := s.Var("heximage")
	err = board.FlashImage(ctx, heximage)
	if err != nil {
		s.Fatal("FlashImage: ", err)
	}

	i := ti50.NewCrOSImage(board)

	if err := i.WaitUntilBooted(ctx); err != nil {
		s.Fatal("WaitUntilBooted after spiflash: ", err)
	}

	out, err := i.Command(ctx, "version")
	if err != nil {
		s.Fatal("console version: ", err)
	}
	testing.ContextLog(ctx, "version after spiflash: ")
	testing.ContextLog(ctx, out)

	cmd := testexec.CommandContext(ctx, "lsusb", "-d", Ti50USBID, "-v")
	bytes, err := cmd.Output()
	if err != nil {
		s.Fatal("lsusb: ", err)
	}
	out = string(bytes)
	serial, _ := s.Var("serial")
	re := regexp.MustCompile(`iSerial\s+\d\s` + serial)
	m := re.FindStringSubmatch(out)
	if m == nil {
		s.Fatal("lsusb serial: ", out)
	}

	cmd = testexec.CommandContext(ctx, "/usr/sbin/gsctool", "-n", serial, "-f")
	if err := cmd.Run(); err != nil {
		s.Fatal("gsctool version: ", err)
	}

	binimage, _ := s.Var("binimage")
	cmd = testexec.CommandContext(ctx, "ls", binimage)
	if err := cmd.Run(); err != nil {
		s.Fatal("binimage: ", err)
	}

	// Ti50 will reject updates for 60 seconds.
	testing.Sleep(ctx, 65*time.Second)

	cmd = testexec.CommandContext(ctx, "/usr/sbin/gsctool", "-n", serial, binimage)
	err = cmd.Run()
	// gsctool exit code is 1 for successful update.
	if c, _ := testexec.ExitCode(err); c != 1 {
		s.Fatal("gsctool update: ", err)
	}

	if err := i.WaitUntilBooted(ctx); err != nil {
		s.Fatal("WaitUntilBooted after gsctool: ", err)
	}

	out, err = i.Command(ctx, "version")
	if err != nil {
		s.Fatal("console version: ", err)
	}
	testing.ContextLog(ctx, "version after gsctool: ")
	testing.ContextLog(ctx, out)
}

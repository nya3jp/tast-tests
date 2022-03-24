// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/common/firmware/ti50"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/ti50/fixture"
	"chromiumos/tast/rpc"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:    Ti50CCDUpdate,
		Desc:    "Ti50 firmware update over CCD using gsctool",
		Timeout: 5 * time.Minute,
		Vars:    []string{"serial"},
		Contacts: []string{
			"ecgh@chromium.org",
			"ti50-core@google.com",
		},
		Attr:    []string{"group:firmware"},
		Fixture: fixture.Ti50,
	})
}

func Ti50CCDUpdate(ctx context.Context, s *testing.State) {
	const Ti50USBID = "18d1:504a"

	f := s.FixtValue().(*fixture.Value)

	board, err := f.DevBoard(ctx, 10000, time.Second)
	if err != nil {
		s.Fatal("Could not get board: ", err)
	}

	dutImage, err := prepareCcdImageFile(ctx, s, f.ImagePath)
	if err != nil {
		s.Fatal("Prepare file: ", err)
	}

	if err = board.Reset(ctx); err != nil {
		s.Fatal("Failed to reset: ", err)
	}

	i := ti50.NewCrOSImage(board)

	testing.Sleep(ctx, 1*time.Second)

	outStr, err := i.Command(ctx, "version")
	if err != nil {
		s.Fatal("Console version: ", err)
	}
	testing.ContextLog(ctx, "Version before gsctool update: ")
	testing.ContextLog(ctx, outStr)

	// Before update should be running RW_A.
	re := regexp.MustCompile(`RW_A:  \* [0-9.]+/ti50_common:v.*[\r\n]+RW_B:    Empty`)
	m := re.FindStringSubmatch(outStr)
	if m == nil {
		s.Fatal("Not running RW_A")
	}

	cmd := s.DUT().Conn().CommandContext(ctx, "lsusb", "-d", Ti50USBID, "-v")
	out, err := cmd.CombinedOutput()
	outStr = string(out)
	if err != nil {
		s.Fatal("Failed lsusb: ", err, outStr)
	}
	serial := s.RequiredVar("serial")
	re = regexp.MustCompile(`iSerial\s+\d\s` + serial)
	m = re.FindStringSubmatch(outStr)
	if m == nil {
		s.Fatal("Failed to match serial: ", outStr)
	}

	cmd = s.DUT().Conn().CommandContext(ctx, "/usr/sbin/gsctool", "-n", serial, "-f")
	out, err = cmd.CombinedOutput()
	outStr = string(out)
	if err != nil {
		s.Fatal("Failed to read version: ", err, outStr)
	}

	// Ti50 will reject updates for 60 seconds.
	testing.Sleep(ctx, 30*time.Second)

	cmd = s.DUT().Conn().CommandContext(ctx, "/usr/sbin/gsctool", "-n", serial, dutImage)
	out, err = cmd.CombinedOutput()
	outStr = string(out)
	re = regexp.MustCompile(`Error: status 0x9`)
	m = re.FindStringSubmatch(outStr)
	if m == nil {
		s.Fatal("Wrong gsctool output for update too soon: ", err, outStr)
	}

	_, err = board.ReadSerialSubmatch(ctx, regexp.MustCompile("Attempted update too soon"))
	if err != nil {
		s.Fatal("Wrong console message for update too soon: ", err)
	}

	testing.Sleep(ctx, 30*time.Second)

	cmd = s.DUT().Conn().CommandContext(ctx, "/usr/sbin/gsctool", "-n", serial, dutImage)
	out, err = cmd.CombinedOutput()
	outStr = string(out)
	re = regexp.MustCompile(`image updated`)
	m = re.FindStringSubmatch(outStr)
	if m == nil {
		s.Fatal("Wrong gsctool output for update: ", err, outStr)
	}

	testing.Sleep(ctx, 1*time.Second)

	outStr, err = i.Command(ctx, "version")
	if err != nil {
		s.Fatal("Console version: ", err)
	}
	testing.ContextLog(ctx, "Version after gsctool update: ")
	testing.ContextLog(ctx, outStr)

	// After update should be running RW_B.
	re = regexp.MustCompile(`RW_B:  \* [0-9.]+/ti50_common:v.*`)
	m = re.FindStringSubmatch(outStr)
	if m == nil {
		s.Fatal("Not running RW_B")
	}
}

func prepareCcdImageFile(ctx context.Context, s *testing.State, image string) (string, error) {
	if image == "" {
		return "", errors.New("no image file")
	}

	rpcClient, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		return "", err
	}
	defer rpcClient.Close(ctx)

	dutfsClient := dutfs.NewClient(rpcClient.Conn)

	workDir, err := dutfsClient.TempDir(ctx, "", "")
	if err != nil {
		return "", err
	}

	dutImage := filepath.Join(workDir, "ti50.bin")

	testing.ContextLogf(ctx, "Copy image %s to DUT %s", image, dutImage)

	_, err = linuxssh.PutFiles(ctx, s.DUT().Conn(), map[string]string{image: dutImage}, linuxssh.DereferenceSymlinks)
	if err != nil {
		return "", err
	}

	// Zero the signature field (offsets 4-100) in RO_A and RO_B since we
	// are using node locked RO on Andreiboard for testing (b/215718883).

	cmd := s.DUT().Conn().CommandContext(ctx, "dd", "seek=4", "count=96", "bs=1", "conv=nocreat,notrunc", "if=/dev/zero", "of="+dutImage)
	if err := cmd.Run(); err != nil {
		return "", err
	}

	cmd = s.DUT().Conn().CommandContext(ctx, "dd", "seek=524292", "count=96", "bs=1", "conv=nocreat,notrunc", "if=/dev/zero", "of="+dutImage)
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return dutImage, nil
}

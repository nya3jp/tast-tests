// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
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

const ti50USBID = "18d1:504a"

var consoleUpdateTooSoonRegexp = regexp.MustCompile("Attempted update too soon")
var gsctoolUpdateTooSoonRegexp = regexp.MustCompile(`Error: status 0x9`)
var gsctoolUpdateSuccessRegexp = regexp.MustCompile(`image updated`)
var versionRwARegexp = regexp.MustCompile(`RW_A:  \* [0-9.]+/ti50_common:v.*[\r\n]+RW_B:    Empty`)
var versionRwBRegexp = regexp.MustCompile(`RW_B:  \* [0-9.]+/ti50_common:v.*`)

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

// Ti50CCDUpdate requires HW setup with SuzyQ cable from Andreiboard to DUT.
func Ti50CCDUpdate(ctx context.Context, s *testing.State) {
	serial := s.RequiredVar("serial")
	lsusbSerialRegexp := regexp.MustCompile(`iSerial\s+\d\s` + serial)
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

	// Wait for reboot output to finish before reading version.
	testing.Sleep(ctx, 1*time.Second)

	outStr, err := i.Command(ctx, "version")
	if err != nil {
		s.Fatal("Console version: ", err)
	}
	testing.ContextLog(ctx, "Version before gsctool update: ")
	testing.ContextLog(ctx, outStr)

	// Before update should be running RW_A.
	if !versionRwARegexp.MatchString(outStr) {
		s.Fatal("Not running RW_A")
	}

	// Wait one more second to ensure that USB is connected before running lsusb on host
	testing.Sleep(ctx, 1*time.Second)

	cmd := s.DUT().Conn().CommandContext(ctx, "lsusb", "-d", ti50USBID, "-v")
	out, err := cmd.CombinedOutput()
	outStr = string(out)
	if err != nil {
		// Report the current usb connection state since lsusb just failed
		outStr, err := i.Command(ctx, "usb")
		if err != nil {
			s.Fatal("Getting ti50 usb state: ", err)
		}
		testing.ContextLog(ctx, "USB state on ti50:")
		testing.ContextLog(ctx, outStr)

		s.Fatal("Failed lsusb: ", err, outStr)
	}
	if !lsusbSerialRegexp.MatchString(outStr) {
		s.Fatal("Failed to match serial: ", outStr)
	}

	cmd = s.DUT().Conn().CommandContext(ctx, "/usr/sbin/gsctool", "-n", serial, "-f")
	out, err = cmd.CombinedOutput()
	if err != nil {
		s.Fatalf("Failed to read version: %v: %s", err, out)
	}

	// Ti50 will reject updates for 60 seconds.
	testing.Sleep(ctx, 30*time.Second)

	cmd = s.DUT().Conn().CommandContext(ctx, "/usr/sbin/gsctool", "-n", serial, dutImage)
	out, _ = cmd.CombinedOutput()
	if !gsctoolUpdateTooSoonRegexp.Match(out) {
		s.Fatalf("Wrong gsctool output for update too soon: %s", out)
	}

	_, err = board.ReadSerialSubmatch(ctx, consoleUpdateTooSoonRegexp)
	if err != nil {
		s.Fatal("Wrong console message for update too soon: ", err)
	}

	testing.Sleep(ctx, 30*time.Second)

	cmd = s.DUT().Conn().CommandContext(ctx, "/usr/sbin/gsctool", "-n", serial, dutImage)
	out, _ = cmd.CombinedOutput()
	if !gsctoolUpdateSuccessRegexp.Match(out) {
		s.Fatalf("Wrong gsctool output for update: %s", out)
	}

	// Wait for reboot output to finish before reading version.
	testing.Sleep(ctx, 1*time.Second)

	outStr, err = i.Command(ctx, "version")
	if err != nil {
		s.Fatal("Console version: ", err)
	}
	testing.ContextLog(ctx, "Version after gsctool update: ")
	testing.ContextLog(ctx, outStr)

	// After update should be running RW_B.
	if !versionRwBRegexp.MatchString(outStr) {
		s.Fatal("Not running RW_B")
	}
}

// prepareCcdImageFile copies the Ti50 image file to the DUT and zeros the RO
// signature field. Returns the DUT file path to be used by gsctool to perform
// the CCD update.
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

	// Zero the signature field (offset 4, length 384) in RO_A and RO_B, and
	// zero the cryptolib magic (offset 0, length 4), since we are using node
	// locked RO on Andreiboard for testing (b/230341252).
	for _, base := range []int{0, 0x800, 0x80000, 0x80800} {
		seek := base + 4
		count := 384
		if base == 0x800 || base == 0x80800 {
			seek = base
			count = 4
		}
		cmd := s.DUT().Conn().CommandContext(ctx, "dd", fmt.Sprintf("seek=%d", seek), fmt.Sprintf("count=%d", count), "bs=1", "conv=nocreat,notrunc", "if=/dev/zero", "of="+dutImage)
		if err := cmd.Run(); err != nil {
			return "", err
		}
	}

	return dutImage, nil
}

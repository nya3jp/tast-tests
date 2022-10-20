// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"regexp"
	"time"

	"chromiumos/tast/common/firmware/ti50"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/remote/firmware/ti50/fixture"
	"chromiumos/tast/testing"
)

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
		Contacts: []string{
			"ecgh@chromium.org",
			"ti50-core@google.com",
		},
		Attr:    []string{"group:firmware"},
		Fixture: fixture.Ti50,
	})
}

// Ti50CCDUpdate requires HW setup with SuzyQ cable from Andreiboard to drone/workstation.
func Ti50CCDUpdate(ctx context.Context, s *testing.State) {
	f := s.FixtValue().(*fixture.Value)

	board, err := f.DevBoard(ctx, 10000, time.Second)
	if err != nil {
		s.Fatal("Could not get board: ", err)
	}

	ccdImage, err := prepareCcdImageFile(ctx, s, f.ImagePath)
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

	// Wait one more second to ensure that USB is connected before running gsctool
	testing.Sleep(ctx, 1*time.Second)

	out, err := board.GSCToolCommand(ctx, "", "--fwver")
	if err != nil {
		// Report the current usb connection state on failure.
		usbOut, err := i.Command(ctx, "usb")
		if err != nil {
			s.Fatal("Getting usb state: ", err)
		}
		testing.ContextLog(ctx, "USB state:")
		testing.ContextLog(ctx, usbOut)
		s.Fatal("Failed to read version: ", err, out)
	}

	// Ti50 will reject updates for 60 seconds.
	testing.Sleep(ctx, 30*time.Second)

	out, _ = board.GSCToolCommand(ctx, ccdImage)
	if !gsctoolUpdateTooSoonRegexp.Match(out) {
		s.Fatalf("Wrong gsctool output for update too soon: %s", out)
	}

	_, err = board.ReadSerialSubmatch(ctx, consoleUpdateTooSoonRegexp)
	if err != nil {
		s.Fatal("Wrong console message for update too soon: ", err)
	}

	testing.Sleep(ctx, 30*time.Second)

	out, _ = board.GSCToolCommand(ctx, ccdImage)
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

// prepareCcdImageFile copies the Ti50 image and zeros the RO signature field.
// Returns the file path to be used by gsctool to perform the CCD update.
func prepareCcdImageFile(ctx context.Context, s *testing.State, image string) (string, error) {
	if image == "" {
		return "", errors.New("no image file")
	}

	f, err := ioutil.TempFile("", "ccd_")
	if err != nil {
		return "", errors.Wrap(err, "create temp image file")
	}
	f.Close()
	ccdImage := f.Name()

	if err := fsutil.CopyFile(image, ccdImage); err != nil {
		return "", errors.Wrap(err, "copy image file")
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
		cmd := exec.CommandContext(ctx, "dd", fmt.Sprintf("seek=%d", seek), fmt.Sprintf("count=%d", count), "bs=1", "conv=nocreat,notrunc", "if=/dev/zero", "of="+ccdImage)
		if err := cmd.Run(); err != nil {
			return "", errors.Wrap(err, "zero with dd")
		}
	}

	return ccdImage, nil
}

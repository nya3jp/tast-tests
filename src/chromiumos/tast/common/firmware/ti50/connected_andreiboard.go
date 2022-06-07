// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ti50

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/firmware/serial"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// ConnectedAndreiboard is one that is directly connected via serial port to
// the localhost.  Localhost being the dut for a test in the local bundle and
// the drone/workstation for one that's in the remote bundle.
type ConnectedAndreiboard struct {
	*Andreiboard
}

// ParseUltraDebugTargets extracts list of UltraDebug target ports from device
// file listings.
func ParseUltraDebugTargets(devs string) []string {
	re := regexp.MustCompile(`/dev/ttyUltraTarget_\S*`)
	//Use for testing
	//re := regexp.MustCompile(`/dev/pts/5`)
	return re.FindAllString(devs, -1)
}

// ListConnectedUltraDebugTargets finds list of UltraDebug targets connected.
func ListConnectedUltraDebugTargets(ctx context.Context) ([]string, error) {
	cmd := testexec.CommandContext(ctx, "find", "/dev/")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return ParseUltraDebugTargets(string(output)), nil
}

// NewConnectedAndreiboard creates a ConnectedAndreiboard.
// bufSize is the max number of expected un-read bytes.
// spiFlash is the path to the spiflash executable.
// readTimeout is the max duration of expected silence during reads.
//
// Example:
//   device, _ := ListConnectedUltraDebugTargets(ctx)
//   if len(device) == 0 {
//       testing.ContextLog(ctx, "Could not find any UD device")
//   }
//   board := NewConnectedAndreiboard(device, 4096, "/path/to/spi/flash", 1 * time.Second)
func NewConnectedAndreiboard(targetDevice string, bufSize int, spiFlash string, readTimeout time.Duration) *ConnectedAndreiboard {
	opener := serial.NewConnectedPortOpener(targetDevice, 115200, readTimeout)
	ab := NewAndreiboard(bufSize, opener, spiFlash)

	return &ConnectedAndreiboard{Andreiboard: ab}
}

// FlashImage flashes an image to the board.
func (a *ConnectedAndreiboard) FlashImage(ctx context.Context, image string) error {
	if a.GetSpiFlash() == "" {
		return errors.New("spiflash binary not provided")
	}

	cmd := testexec.CommandContext(ctx, "ls", a.GetSpiFlash())
	if err := cmd.Run(); err != nil {
		return errors.New("spiflash not found: " + a.GetSpiFlash())
	}

	cmd = testexec.CommandContext(ctx, "ls", image)
	if err := cmd.Run(); err != nil {
		return errors.New("image not found: " + image)
	}

	cmd = testexec.CommandContext(ctx, a.GetSpiFlash(), "--dauntless", "--tty=2", "--verbose", "--extraverbose", "--input="+image)

	out, err := cmd.CombinedOutput()

	if err != nil {
		testing.ContextLogf(ctx, "Flash failed: %v, Output below:", err)
		testing.ContextLog(ctx, string(out))
	}

	return err
}

// OpenTitanToolCommand runs an arbitrary OpenTitan tool command (without up-/downloading any files).
func (a *ConnectedAndreiboard) OpenTitanToolCommand(ctx context.Context, cmd string, args ...string) (output []byte, err error) {
	return nil, errors.New("Unimplemented ConnectedAndreiboard.OpenTitanToolCommand")
}

// Reset causes the board FlashImage flashes an image to the board.
func (a *ConnectedAndreiboard) Reset(ctx context.Context) error {
	if a.GetSpiFlash() == "" {
		return errors.New("spiflash binary not provided")
	}

	cmd := testexec.CommandContext(ctx, a.GetSpiFlash(), "--dauntless", "--just_reset")

	out, err := cmd.Output()

	if err != nil {
		testing.ContextLogf(ctx, "Reset failed: %v, Output below:", err)
		testing.ContextLog(ctx, out)
	}

	return err
}

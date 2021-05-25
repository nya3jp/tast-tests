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
)

type ConnectedAndreiBoard struct {
	*AndreiBoard
        spiFlash              string
}

func ParseUltraDebugTargets(devs string) []string {
	re := regexp.MustCompile(`/dev/ttyUltraTarget_\S*`)
	//Use for testing
	//re := regexp.MustCompile(`/dev/pts/5`)
	return re.FindAllString(devs, -1)
}

func ListConnectedUltraDebugTargets(ctx context.Context) ([]string, error) {
	cmd := testexec.CommandContext(ctx, "find", "/dev/")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return ParseUltraDebugTargets(string(output)), nil
}

func NewConnectedAndreiBoard(targetDevice string, bufSize int, spiFlash string, readTimeout time.Duration) *ConnectedAndreiBoard {
	opener := serial.NewConnectedPortOpener(targetDevice, 115200, readTimeout)
	ab := NewAndreiBoard(bufSize, opener)

	return &ConnectedAndreiBoard{AndreiBoard: ab, spiFlash: spiFlash}
}

func (a *ConnectedAndreiBoard) FlashImage(ctx context.Context, image string) error {
	if a.spiFlash == "" {
		return errors.New("spiflash binary not provided")
	}

	cmd := testexec.CommandContext(ctx, "ls", a.spiFlash)
	if err := cmd.Run(); err != nil {
		return errors.New("spiflash not found: " + a.spiFlash)
	}

	cmd = testexec.CommandContext(ctx, "ls", image)
	if err := cmd.Run(); err != nil {
		return errors.New("Image not found: " + image)
	}

	cmd = testexec.CommandContext(ctx, a.spiFlash, "--dauntless", "--tty=2", "--verbose", "-X", "--input="+image)

	err := cmd.Run()

	return err
}

func (a *ConnectedAndreiBoard) Reset(ctx context.Context) error {
	if a.spiFlash == "" {
		return errors.New("spiflash binary not provided")
	}

	cmd := testexec.CommandContext(ctx, a.spiFlash, "-d", "-j")
	return cmd.Run()
}

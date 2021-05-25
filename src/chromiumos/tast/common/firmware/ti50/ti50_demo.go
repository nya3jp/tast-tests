// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ti50

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func CreateConnectedDemoBoard(ctx context.Context, spiflash string) (DevBoard, error) {
	targets, err := ListConnectedUltraDebugTargets(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Error finding UD targets")
	} else if len(targets) == 0 {
		return nil, errors.New("No UD targets found on device")
	} else {
		testing.ContextLogf(ctx, "UD Targets: %v", targets)
	}

	tty := string(targets[0])

	return NewConnectedAndreiBoard(tty, 4096, spiflash, time.Microsecond*1), nil
}

// image and spiflash are optional, will demo on existing image.
func Demo(ctx context.Context, board DevBoard, image string, spiflash string) error {
	if spiflash != "" && image != "" {
		testing.ContextLogf(ctx, "Flashing %s with %s", image, spiflash)
		if err := board.FlashImage(ctx, image); err != nil {
			return errors.Wrap(err, "Failed to flash image")
		}
		testing.ContextLog(ctx, "Flashing finished")
	} else if false && spiflash != "" {
		testing.ContextLogf(ctx, "Resetting board with %s", spiflash)
		if err := board.Reset(ctx); err != nil {
			return errors.Wrap(err, "Failed to reset board")
		}
		testing.ContextLog(ctx, "Reset finished")
	} else {
		testing.ContextLog(ctx, "Running demo without resetting board")
	}

	i := NewTi50Image(board)

	if err := i.WaitUntilBooted(ctx); err != nil {
		return errors.Wrap(err, "Failed to wait for boot on ti50 image")
	}
	testing.ContextLog(ctx, "Board has booted")

	var anyErr error

	testing.ContextLogf(ctx, "Starting bid demo")
	if err := DemoCommand(ctx, i, "bid"); err != nil {
		anyErr = errors.Wrap(err, "bid demo failed")
		testing.ContextLog(ctx, anyErr)
	}

	testing.ContextLogf(ctx, "Starting sysinfo demo")
	if err := DemoCommand(ctx, i, "sysinfo"); err != nil {
		anyErr = errors.Wrap(err, "sysinfo demo failed")
		testing.ContextLog(ctx, anyErr)
	}

	testing.ContextLogf(ctx, "Starting ccdstate demo")
	ccdstateRe := regexp.MustCompile(`(?s)AP:\s*(?P<AP>\w*)\s*Servo:\s*(?P<Servo>\w*)\s*Rdd:\s*(\w*)`)
	if err := DemoRawCommand(ctx, i, "ccdstate", ccdstateRe); err != nil {
		anyErr = errors.Wrap(err, "ccdstate demo failed")
		testing.ContextLog(ctx, anyErr)
	}

	testing.ContextLogf(ctx, "Starting help demo")
	if err := DemoHelp(ctx, i); err != nil {
		anyErr = errors.Wrap(err, "help demo failed")
		testing.ContextLog(ctx, anyErr)
	}

	return anyErr
}

func DemoHelp(ctx context.Context, i *Ti50Image) error {
	if err := i.GetPrompt(ctx); err != nil {
		return err
	}
	h, err := i.Help(ctx)
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "Help Output: %s", h.Raw)
	testing.ContextLogf(ctx, "Help commands: %v", h.Commands)

	return nil
}

func DemoCommand(ctx context.Context, i *Ti50Image, cmd string) error {
	if err := i.GetPrompt(ctx); err != nil {
		return err
	}
	out, err := i.Command(ctx, cmd)
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "%s Output: %s", cmd, out)

	return nil
}

func DemoRawCommand(ctx context.Context, i *Ti50Image, cmd string, re *regexp.Regexp) error {
	if err := i.GetPrompt(ctx); err != nil {
		return err
	}
	m, err := i.RawCommand(ctx, cmd+"\r", re)
	if err != nil {
		return err
	}
	for i, n := range re.SubexpNames() {
		if n == "" {
			testing.ContextLogf(ctx, "Field %d has value: %s", i, m[i])
		} else {
			testing.ContextLogf(ctx, "Field %s has value: %s", n, m[i])
		}
	}

	return nil
}

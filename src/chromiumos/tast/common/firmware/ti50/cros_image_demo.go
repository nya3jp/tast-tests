// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ti50

import (
	"context"
	"regexp"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Demo uses some of the CrOSImage to control the board.  Image is optional,
// will demo on existing image if set to "".
func Demo(ctx context.Context, board DevBoard, image string) error {
	if image != "" {
		testing.ContextLog(ctx, "Flashing ", image)
		if err := board.FlashImage(ctx, image); err != nil {
			return errors.Wrap(err, "failed to flash image")
		}
		testing.ContextLog(ctx, "Flashing finished")
	} else {
		testing.ContextLog(ctx, "Running demo without flashing")
	}

	i := NewCrOSImage(board)

	if err := i.WaitUntilBooted(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for boot on ti50 image")
	}
	testing.ContextLog(ctx, "Board has booted")

	var anyErr error

	testing.ContextLog(ctx, "Starting bid demo")
	if err := DemoCommand(ctx, i, "bid"); err != nil {
		anyErr = errors.Wrap(err, "bid demo failed")
		testing.ContextLog(ctx, anyErr)
	}

	testing.ContextLog(ctx, "Starting sysinfo demo")
	if err := DemoCommand(ctx, i, "sysinfo"); err != nil {
		anyErr = errors.Wrap(err, "sysinfo demo failed")
		testing.ContextLog(ctx, anyErr)
	}

	testing.ContextLog(ctx, "Starting ccdstate demo")
	ccdstateRe := regexp.MustCompile(`(?s)AP:\s*(?P<AP>\w*)\s*Servo:\s*(?P<Servo>\w*)\s*Rdd:\s*(\w*)`)
	if err := DemoRawCommand(ctx, i, "ccdstate", ccdstateRe); err != nil {
		anyErr = errors.Wrap(err, "ccdstate demo failed")
		testing.ContextLog(ctx, anyErr)
	}

	testing.ContextLog(ctx, "Starting help demo")
	if err := DemoHelp(ctx, i); err != nil {
		anyErr = errors.Wrap(err, "help demo failed")
		testing.ContextLog(ctx, anyErr)
	}

	return anyErr
}

// DemoHelp demos the Help method.
func DemoHelp(ctx context.Context, i *CrOSImage) error {
	if err := i.GetPrompt(ctx); err != nil {
		return err
	}
	h, err := i.Help(ctx)
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "Help Output: %s", h.Raw)
	testing.ContextLog(ctx, "Help commands: ", h.Commands)

	return nil
}

// DemoCommand demos the Command method.
func DemoCommand(ctx context.Context, i *CrOSImage, cmd string) error {
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

// DemoRawCommand demos the RawCommand method.
func DemoRawCommand(ctx context.Context, i *CrOSImage, cmd string, re *regexp.Regexp) error {
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

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/errors"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

// SetU2fdFlags sets the flags and restarts u2fd, which will re-create the u2f device.
func SetU2fdFlags(ctx context.Context, helper *hwsecremote.FullHelperRemote, u2f, g2f, userKeys bool) (retErr error) {
	const (
		uf2ForcePath      = "/var/lib/u2f/force/u2f.force"
		gf2ForcePath      = "/var/lib/u2f/force/g2f.force"
		userKeysForcePath = "/var/lib/u2f/force/user_keys.force"
	)

	cmd := helper.CmdRunner()
	dCtl := helper.DaemonController()

	if err := dCtl.Stop(ctx, hwsec.U2fdDaemon); err != nil {
		return errors.Wrap(err, "failed to stop u2fd")
	}
	defer func(ctx context.Context) {
		if err := dCtl.Start(ctx, hwsec.U2fdDaemon); err != nil {
			if retErr != nil {
				testing.ContextLog(ctx, "Failed to restart u2fd: ", err)
			} else {
				retErr = errors.Wrap(err, "failed to restart u2fd")
			}
		}
	}(ctx)

	// Remove flags.
	if _, err := cmd.Run(ctx, "sh", "-c", "rm -f /var/lib/u2f/force/*.force"); err != nil {
		return errors.Wrap(err, "failed to remove flags")
	}
	if u2f {
		if _, err := cmd.Run(ctx, "touch", uf2ForcePath); err != nil {
			return errors.Wrap(err, "failed to set u2f flag")
		}
	}
	if g2f {
		if _, err := cmd.Run(ctx, "touch", gf2ForcePath); err != nil {
			return errors.Wrap(err, "failed to set g2f flag")
		}
	}
	if userKeys {
		if _, err := cmd.Run(ctx, "touch", userKeysForcePath); err != nil {
			return errors.Wrap(err, "failed to set userKeys flag")
		}
	}
	return nil
}

// EnsureChapsSlotsInitialized ensures chaps is initialized.
func EnsureChapsSlotsInitialized(ctx context.Context, chaps *pkcs11.Chaps) error {
	return testing.Poll(ctx, func(context.Context) error {
		slots, err := chaps.ListSlots(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to list chaps slots")
		}
		testing.ContextLog(ctx, slots)
		if len(slots) < 2 {
			return errors.Wrap(err, "chaps initialization hasn't finished")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  30 * time.Second,
		Interval: time.Second,
	})
}

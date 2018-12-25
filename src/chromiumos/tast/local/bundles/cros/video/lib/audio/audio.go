// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package audio mute/unmute DUT.
package audio

import (
	"context"

	"chromiumos/tast/local/testexec"
)

// Mute let DUT be mute. For example, after Mute() is done, DUT doesn't sounds when a video plays.
func Mute(ctx context.Context) error {
	cmd := testexec.CommandContext(ctx, "cras_test_client", "--mute", "1")
	if err := cmd.Start(); err != nil {
		cmd.DumpLog(ctx)
		return err
	}
	return nil
}

// Unmute let DUT be unmute, if it is muted by Mute().
func Unmute(ctx context.Context) error {
	cmd := testexec.CommandContext(ctx, "cras_test_client", "--mute", "0")
	if err := cmd.Start(); err != nil {
		cmd.DumpLog(ctx)
		return err
	}
	return nil
}

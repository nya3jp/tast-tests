// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crastestclient provides functions to interact cras_test_client
package crastestclient

import (
	"context"

	"chromiumos/tast/common/testexec"
)

// Mute lets DUT be muted. That is, after Mute() is done, DUT doesn't sound when a video plays.
func Mute(ctx context.Context) error {
	cmd := testexec.CommandContext(ctx, "cras_test_client", "--mute", "1")
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return err
	}
	return nil
}

// Unmute lets DUT be unmuted, if it is muted by Mute().
func Unmute(ctx context.Context) error {
	cmd := testexec.CommandContext(ctx, "cras_test_client", "--mute", "0")
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return err
	}
	return nil
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ping contains utility functions to wrap around the ping program.
package ping

import (
	"chromiumos/tast/common/network/cmd"
	"chromiumos/tast/common/network/ping"
)

var _ cmd.Runner = (*cmd.LocalCmdRunner)(nil)

// Runner is an alias for common ping Runner but only for local execution.
type Runner = ping.Runner

// NewRunner creates a ping Runner on the given dut for local execution.
func NewRunner() *Runner {
	return ping.NewRunner(&cmd.LocalCmdRunner{})
}

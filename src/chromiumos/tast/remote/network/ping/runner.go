// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ping contains utility functions to wrap around the ping program.
package ping

import (
	"chromiumos/tast/common/network/cmd"
	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/remote/network/commander"
)

// Option is an alias for for common ping Runner.
type Option = ping.Option

var _ cmd.Runner = (*cmd.RemoteCmdRunner)(nil)

// Runner is an alias for common ping Runner but only for remote execution.
type Runner = ping.Runner

// NewRunner creates a ping Runner on the given dut for remote execution.
func NewRunner(host commander.Commander) *Runner {
	return ping.NewRunner(&cmd.RemoteCmdRunner{Host: host})
}

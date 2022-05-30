// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ip contains utility functions to wrap around the ip program.
package ip

import (
	"go.chromium.org/chromiumos/tast-tests/common/network/ip"
	"go.chromium.org/chromiumos/tast-tests/remote/network/cmd"
	"go.chromium.org/chromiumos/tast/ssh"
)

// Runner is an alias for common ip Runner but only for remote execution.
type Runner = ip.Runner

// NewRemoteRunner creates a ip runner for remote execution.
func NewRemoteRunner(host *ssh.Conn) *Runner {
	return ip.NewRunner(&cmd.RemoteCmdRunner{Host: host})
}

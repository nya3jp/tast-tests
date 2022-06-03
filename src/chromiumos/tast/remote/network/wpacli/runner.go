// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wpacli contains utility functions to wrap around the wpacli program.
package wpacli

import (
	"chromiumos/tast/common/network/wpacli"
	"chromiumos/tast/remote/network/cmd"
	"chromiumos/tast/ssh"
)

// Runner is an alias for common wpacli Runner but only for remote execution.
type Runner = wpacli.Runner

// NewRemoteRunner creates a wpacli runner for remote execution.
func NewRemoteRunner(host *ssh.Conn) *Runner {
	return wpacli.NewRunner(&cmd.RemoteCmdRunner{Host: host})
}

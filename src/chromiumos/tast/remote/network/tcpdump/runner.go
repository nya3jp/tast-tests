// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tcpdump

import (
	"chromiumos/tast/common/network/tcpdump"
	"chromiumos/tast/remote/network/cmd"
	"chromiumos/tast/ssh"
)

// Runner is an alias for common tcpdump Runner but only for remote execution.
type Runner = tcpdump.Runner

// NewRemoteRunner creates an tcpdump runner for remote execution.
func NewRemoteRunner(host *ssh.Conn) *Runner {
	return tcpdump.NewRunner(&cmd.RemoteCmdRunner{Host: host})
}

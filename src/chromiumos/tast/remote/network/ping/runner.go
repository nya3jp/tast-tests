// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ping provides a factory to run ping on DUT from remote machine.
package ping

import (
	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/remote/network/cmd"
	"chromiumos/tast/ssh"
)

// NewRemoteRunner creates a ping Runner on the given dut for remote execution.
func NewRemoteRunner(host *ssh.Conn) *ping.Runner {
	return ping.NewRunner(&cmd.RemoteCmdRunner{Host: host})
}

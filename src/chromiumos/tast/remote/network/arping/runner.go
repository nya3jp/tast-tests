// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arping provides a factory to run arping on DUT from remote machine.
package arping

import (
	"chromiumos/tast/common/network/arping"
	"chromiumos/tast/remote/network/cmd"
	"chromiumos/tast/ssh"
)

// NewRemoteRunner creates a arping Runner on the given dut for remote execution.
func NewRemoteRunner(host *ssh.Conn) *arping.Runner {
	return arping.NewRunner(&cmd.RemoteCmdRunner{Host: host})
}

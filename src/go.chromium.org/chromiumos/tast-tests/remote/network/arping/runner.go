// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arping provides a factory to run arping on DUT from remote machine.
package arping

import (
	"go.chromium.org/chromiumos/tast-tests/common/network/arping"
	"go.chromium.org/chromiumos/tast-tests/remote/network/cmd"
	"go.chromium.org/chromiumos/tast/ssh"
)

// NewRemoteRunner creates an arping Runner on the given dut for remote execution.
func NewRemoteRunner(host *ssh.Conn) *arping.Runner {
	return arping.NewRunner(&cmd.RemoteCmdRunner{Host: host})
}

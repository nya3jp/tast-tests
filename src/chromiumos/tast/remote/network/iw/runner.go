// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package iw contains utility functions to wrap around the iw program.
package iw

import (
	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/remote/network/cmd"
	"chromiumos/tast/ssh"
)

// NewRunner creates an iw runner for remote execution.
func NewRunner(host *ssh.Conn) *iw.Runner {
	return iw.NewRunner(&cmd.RemoteCmdRunner{Host: host})
}

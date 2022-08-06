// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package firewall wraps basic iptables call to control
// filtering of incoming/outgoing traffic.
package firewall

import (
	"chromiumos/tast/common/network/firewall"
	"chromiumos/tast/remote/network/cmd"
	"chromiumos/tast/ssh"
)

// Runner is an alias for common firewall Runner but only for remote execution.
type Runner = firewall.Runner

// NewRemoteRunner creates a firewall runner for remote execution.
func NewRemoteRunner(host *ssh.Conn) *Runner {
	return firewall.NewRunner(&cmd.RemoteCmdRunner{Host: host})
}

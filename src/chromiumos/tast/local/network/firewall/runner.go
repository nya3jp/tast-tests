// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package firewall wraps basic iptables call to control
// filtering of incoming/outgoing traffic.
package firewall

import (
	"chromiumos/tast/common/network/firewall"
	"chromiumos/tast/local/network/cmd"
)

// Runner is an alias for common firewall Runner but only for local execution.
type Runner = firewall.Runner

// NewLocalRunner creates an firewall runner for local execution.
func NewLocalRunner() *Runner {
	return firewall.NewRunner(&cmd.LocalCmdRunner{})
}

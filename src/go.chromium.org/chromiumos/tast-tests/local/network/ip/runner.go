// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ip contains utility functions to wrap around the ip program.
package ip

import (
	"go.chromium.org/chromiumos/tast-tests/common/network/ip"
	"go.chromium.org/chromiumos/tast-tests/local/network/cmd"
)

// Runner is an alias for common ip Runner but only for local execution.
type Runner = ip.Runner

// NewLocalRunner creates an ip runner for local execution.
func NewLocalRunner() *Runner {
	return ip.NewRunner(&cmd.LocalCmdRunner{})
}

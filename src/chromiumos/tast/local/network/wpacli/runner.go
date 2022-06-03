// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wpacli contains utility functions to wrap around the wpacli program.
package wpacli

import (
	"chromiumos/tast/common/network/wpacli"
	"chromiumos/tast/local/network/cmd"
)

// Runner is an alias for common wpacli Runner but only for local execution.
type Runner = wpacli.Runner

// NewLocalRunner creates an wpacli runner for local execution.
func NewLocalRunner() *Runner {
	return wpacli.NewRunner(&cmd.LocalCmdRunner{})
}

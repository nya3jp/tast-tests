// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ping provides a factory to run ping on DUT.
package ping

import (
	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/local/network/cmd"
)

// NewRunner creates a ping Runner on the given dut for local execution.
func NewRunner() *ping.Runner {
	return ping.NewRunner(&cmd.LocalCmdRunner{})
}

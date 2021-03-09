// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file declares the common interfaces of command runner.
*/

import (
	"context"

	"chromiumos/tast/errors"
)

// CmdRunner declares interface that runs command on DUT.
type CmdRunner interface {
	Run(ctx context.Context, cmd string, args ...string) ([]byte, error)
}

// CmdExitError is the error returned by CmdRunner when the command execution fail.
type CmdExitError struct {
	*errors.E
	ExitCode int
}

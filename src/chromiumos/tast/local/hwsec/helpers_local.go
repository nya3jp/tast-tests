// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file implements miscellaneous and unsorted helpers.
*/

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

type cmdRunnerLocal struct {
}

// Run implements the one of hwsec.CmdRunner.
func (r *cmdRunnerLocal) Run(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	testing.ContextLogf(ctx, "Running: %s", shutil.EscapeSlice(append([]string{cmd}, args...)))
	return testexec.CommandContext(ctx, cmd, args...).Output()
}

// NewHelper creates a new hwsec.Helper instance that make use of the functions
// implemented by cmdRunnerLocal.
func NewHelper() (*hwsec.Helper, error) {
	runner := &cmdRunnerLocal{}
	return &hwsec.Helper{runner}, nil
}

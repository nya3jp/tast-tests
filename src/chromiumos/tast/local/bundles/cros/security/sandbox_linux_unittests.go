// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"

	"chromiumos/tast/local/chrome/bintest"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SandboxLinuxUnittests,
		Desc: "Runs the sandbox_linux_unittests Chrome binary",
		Attr: []string{"informational"},
	})
}

func SandboxLinuxUnittests(ctx context.Context, s *testing.State) {
	const exec = "sandbox_linux_unittests"
	if err := bintest.Run(ctx, exec, nil, s.OutDir()); err != nil {
		s.Errorf("%s failed: %v", exec, err)
	}
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     LocalVarDeps,
		Desc:     "Helper test for testing Tast handles VarDeps",
		Contacts: []string{"oka@chromium.org", "tast-owners@google.com"},
		Vars:     []string{"meta.LocalVarDeps.var"},
		VarDeps:  []string{"meta.LocalVarDeps.var"},
		// This test is called by remote tests in the meta package.
	})
}

func LocalVarDeps(ctx context.Context, s *testing.State) {
	// Do nothing. For the purpose of testing VarDeps, we just want to check
	// whether the test runs or not.
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     AccessVars,
		Desc:     "Access variables",
		Contacts: []string{"tast-owners@google.com", "seewaifu@chromium.org"},
		Attr:     []string{"group:mainline", "informational"},
		// example.AccessVars.foo is defined in tast-tests/vars/example.AccessVars.yaml
		VarDeps: []string{"example.AccessVars.foo"},
	})
}

func AccessVars(ctx context.Context, s *testing.State) {
	varName := "example.AccessVars.foo"
	state, ok := testing.ContextTestState(ctx)
	if !ok {
		s.Error("Got false while trying to get current state from context")
	}
	if s.RequiredVar(varName) != state.RequiredVar(varName) {
		s.Errorf("Got %q for var %q, want %q", state.RequiredVar(varName), varName, s.RequiredVar(varName))
	}
}

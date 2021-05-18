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
	val, ok := testing.ContextVar(ctx, varName)
	expectedVal, expectedOk := s.Var(varName)
	if val != expectedVal || ok != expectedOk {
		s.Errorf("Got (%q, %v) from ContextVar want (%q, %v)", val, ok, expectedVal, expectedOk)
	}
	val = testing.ContextRequiredVar(ctx, varName)
	expectedVal = s.RequiredVar(varName)
	if val != expectedVal {
		s.Errorf("Got %q for ContextRequiredVar, want %q", val, expectedVal)
	}
}

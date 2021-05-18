// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/local/example"
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
		Fixture: "exampleFixtureWithVar",
	})
}

func AccessVars(ctx context.Context, s *testing.State) {
	varName := "example.AccessVars.foo"
	val, ok := testing.ContextVar(ctx, varName)
	expectedVal, expectedOk := s.Var(varName)
	if val != expectedVal || ok != expectedOk {
		s.Errorf("Got (%q, %v) from ContextVar want (%q, %v)", val, ok, expectedVal, expectedOk)
	}
	fixtureVarData := s.FixtValue().(*example.VarData)
	val, ok = testing.ContextVar(ctx, fixtureVarData.Name)
	if val != fixtureVarData.Val || ok != expectedOk {
		s.Errorf("Got fixture variable value (%q, %v) from ContextVar want (%q, %v)", val, ok, fixtureVarData.Val, true)
	}
	val, ok = testing.ContextVar(ctx, "example.AccessVars.globalBoolean")
	if val != "true" || !ok {
		s.Errorf("Got gloval variable value (%q, %v) from ContextVar want (%q, %v)", val, ok, "true", true)
	}
}

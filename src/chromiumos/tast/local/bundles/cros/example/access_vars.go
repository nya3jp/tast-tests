// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/testing"
)

// exampleStrVar demonstrates how to declare a global runtime variable.
var exampleStrVar = testing.NewVarString(
	"example.AccessVars.globalString",
	"An example variable of string type to demonstrate how to use global variable",
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     AccessVars,
		Desc:     "Access variables",
		Contacts: []string{"tast-owners@google.com", "seewaifu@chromium.org"},
		Attr:     []string{"group:mainline", "informational"},
	})
	testing.AddVar(exampleStrVar)
}

func AccessVars(ctx context.Context, s *testing.State) {
	if strVal, ok := exampleStrVar.Value(); strVal != "test" || !ok {
		s.Errorf("Got global variable value (%q, %v) from ContextVar want (%q, %v)", strVal, ok, "test", true)
	}
}

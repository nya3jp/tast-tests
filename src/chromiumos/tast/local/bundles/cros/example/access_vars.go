// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/testing"
)

// exampleStrVar demonstrates how to declare a global runtime variable.
var exampleStrVar = testing.RegisterVarString(
	"example.AccessVars.globalString",
	"Default value",
	"An example variable of string type to demonstrate how to use global variable",
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     AccessVars,
		Desc:     "Access variables",
		Contacts: []string{"tast-owners@google.com", "seewaifu@chromium.org"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func AccessVars(ctx context.Context, s *testing.State) {
	if strVal := exampleStrVar.Value(); strVal != "test" {
		s.Errorf("Got global variable value %q from variable %q want %q", strVal, exampleStrVar.Name(), "test")
	}
}

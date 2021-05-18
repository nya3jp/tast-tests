// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/testing"
)

var (
	// exampleStrVar demonstrates how to declare a runtime variable.
	exampleStrVar = testing.RegisterVarString(
		"example.strvar", // The name of the variable which should have "<pkg_name>." as prefix.
		"Default value",
		"An example variable of string type to demonstrate how to use runtime variable")
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     RuntimeVars,
		Desc:     "Runtime variables",
		Contacts: []string{"tast-owners@google.com", "seewaifu@chromium.org"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func RuntimeVars(ctx context.Context, s *testing.State) {
	testing.ContextLogf(ctx, "Runtime variable %q has value of %q", exampleStrVar.Name(), exampleStrVar.Value())
}

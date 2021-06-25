// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package global defines items that can be used in test libraries or test.
package global

import (
	"chromiumos/tast/testing"
)

// ExampleStrVar demonstrates how to declare a global runtime variable.
var ExampleStrVar = testing.NewVarString(
	"example.AccessVars.globalString",
	"An example variable of string type to demonstrate how to use global variable",
)

// Vars is a list of declared global variables.
var Vars = []testing.Var{
	ExampleStrVar,
}

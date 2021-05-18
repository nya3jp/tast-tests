// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package global defines items that can be used in test libraries or test.
package global

import (
	"chromiumos/tast/testing"
)

// Var define a structure to define a global runtime variable
type Var struct {
	// Name is the name of the variable.
	Name string

	// Desc is a description of the variable.
	Desc string
}

// vars is a list of declared global variables and their descriptions.
var vars = []Var{
	{
		Name: "example.AccessVars.globalBoolean",
		Desc: "An example variable to demonstrate how to use global varaible",
	},
}

func init() {
	var vl []string
	for _, v := range vars {
		vl = append(vl, v.Name)
	}
	testing.AddVars(vl)
}

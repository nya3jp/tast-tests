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

	// Type specifies the value type.
	Type interface{}
}

// The following section should define all global variables that can be used.
const (
	ExampleBoolVar   = "example.AccessVars.globalBoolean" // An example variable of boolean
	ExampleStrVar    = "example.AccessVars.globalString"  // An example variable of string type
	ExampleStructVar = "example.AccessVars.globalStruct"  // An example variable of a struct type
)

// The following section should define all custom types for all global variables that can be used.

// ExampleStruct is used demonstrated the use of struct for the value of a variable.
type ExampleStruct struct {
	Name  string `json:"name,omitempty"`
	Value int    `json:"value,omitempty"`
}

// vars is a list of declared global variables and their descriptions.
var vars = []Var{
	{
		Name: ExampleBoolVar,
		Desc: "An example variable of boolean type to demonstrate how to use global variable",
		Type: false,
	},
	{
		Name: ExampleStrVar,
		Desc: "An example variable of string type to demonstrate how to use global variable",
		Type: "",
	},
	{
		Name: ExampleStructVar,
		Desc: "An example variable of a struct type to demonstrate how to use global variable",
		Type: ExampleStruct{},
	},
}

func init() {
	varTypes := make(map[string]interface{})
	for _, v := range vars {
		varTypes[v.Name] = v.Type
	}
	testing.AddVars(varTypes)
}

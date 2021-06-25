// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package global defines items that can be used in test libraries or test.
package global

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Var define an interface for global runtime variable types.
type Var interface {
	unmarshal(data string)
	Name() string
}

// varBase defines a structure to define a global runtime variable
type varBase struct {
	name     string // name is the name of the variable.
	desc     string // desc is a description of the variable.
	hasValue bool   // hasValue indicates whether the value has been set.
}

func (v *varBase) Name() string {
	return v.name
}

// VarInt define a structure to define a global runtime variable of integer type.
type VarInt struct {
	varBase
	// values store value of the variable
	value int
}

// Value returns value of a variable and a flag to indicate whether the value is initialized.
func (v *VarInt) Value() (int, bool) {
	return v.value, v.hasValue
}

func (v *VarInt) unmarshal(data string) {
	if err := json.Unmarshal([]byte(data), &v.value); err != nil {
		panic(fmt.Sprintf("Failed to unmarshal global variable %q: %v", v.name, err))
	}
	v.hasValue = true
}

// VarBool define a structure to define a global runtime variable of bool type.
type VarBool struct {
	varBase
	// values store value of the variable.
	value bool
}

// Value returns value of a variable and a flag to indicate whether the value is initialized.
func (v *VarBool) Value() (bool, bool) {
	return v.value, v.hasValue
}

func (v *VarBool) unmarshal(data string) {
	if err := json.Unmarshal([]byte(data), &v.value); err != nil {
		panic(fmt.Sprintf("Failed to unmarshal global variable %q: %v", v.name, err))
	}
	v.hasValue = true
}

// VarFloat64 define a structure to define a global runtime variable of float64 type.
type VarFloat64 struct {
	varBase
	// values store value of the variable.
	value float64
}

// Value returns value of a variable and a flag to indicate whether the value is initialized.
func (v *VarFloat64) Value() (float64, bool) {
	return v.value, v.hasValue
}

func (v *VarFloat64) unmarshal(data string) {
	if err := json.Unmarshal([]byte(data), &v.value); err != nil {
		panic(fmt.Sprintf("Failed to unmarshal global variable %q: %v", v.name, err))
	}
	v.hasValue = true
}

// VarString define a structure to define a global runtime variable of string type.
type VarString struct {
	varBase
	// Values store value of the variable.
	value string
}

// Value returns value of a variable and a flag to indicate whether the value is initialized.
func (v *VarString) Value() (string, bool) {
	return v.value, v.hasValue
}

func (v *VarString) unmarshal(data string) {
	v.value = data
	v.hasValue = true
}

// The following section should define all custom types for all global variables that can be used.

// ExampleStruct is used demonstrated the use of struct for the value of a variable.
type ExampleStruct struct {
	Name  string `json:"name,omitempty"`
	Value int    `json:"value,omitempty"`
}

// VarExampleStruct define a structure to define a global runtime variable of VarExampleStruct type.
type VarExampleStruct struct {
	varBase
	// values store value of the variable
	value ExampleStruct
}

// Value returns value of a variable and a flag to indicate whether the value is initialized.
func (v *VarExampleStruct) Value() (*ExampleStruct, bool) {
	return &v.value, v.hasValue
}

func (v *VarExampleStruct) unmarshal(data string) {
	if err := json.Unmarshal([]byte(data), &v.value); err != nil {
		panic(fmt.Sprintf("Failed to unmarshal global variable %q: %v", v.name, err))
	}
	v.hasValue = true
}

// The following section defines all global runtime variables.
var (
	// ExampleBoolVar is an example of a variable of boolean.
	ExampleBoolVar = VarBool{
		varBase: varBase{
			name: "example.AccessVars.globalBoolean",
			desc: "An example variable of boolean type to demonstrate how to use global variable",
		},
		value: false,
	}
	// ExampleStrVar is an example variable of string type.
	ExampleStrVar = VarString{
		varBase: varBase{
			name: "example.AccessVars.globalString",
			desc: "An example variable of string type to demonstrate how to use global variable",
		},
		value: "",
	}
	// ExampleStructVar is an example variable of a struct type.
	ExampleStructVar = VarExampleStruct{
		varBase: varBase{
			name: "example.AccessVars.globalStruct",
			desc: "An example variable of struct type to demonstrate how to use global variable",
		},
		value: ExampleStruct{},
	}
)

// vars is a list of declared global variables.
var vars = []Var{
	&ExampleBoolVar,
	&ExampleStrVar,
	&ExampleStructVar,
}

var initialized bool

// InitializeGlobalVars sets the values for all global variables.
func InitializeGlobalVars(varValues map[string]string) error {
	if initialized {
		return errors.New("cannot initialize global runtime variables twice")
	}
	initialized = true
	for _, v := range vars {
		stringValue, ok := varValues[v.Name()]
		if ok {
			v.unmarshal(stringValue)
		}
	}
	return nil
}

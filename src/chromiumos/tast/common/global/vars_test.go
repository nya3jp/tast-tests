// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package global

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestCheckDuplicateVars makes sure that there are no duplicate variable in globalVars
func TestCheckDuplicateVars(t *testing.T) {
	varNames := make(map[string]struct{})
	for _, v := range vars {
		if _, found := varNames[v.Name()]; found {
			t.Errorf("variable %v is defined more than once", v.Name())
		}
		varNames[v.Name()] = struct{}{}
	}
}

// TestInitializeGlobalVars tests if InitializeGlobalVar behave correctly.
func TestInitializeGlobalVars(t *testing.T) {
	const (
		strValue  = `test value`
		structStr = `{"name":"t1","value":8}`
	)
	structValue := ExampleStruct{
		Name:  "t1",
		Value: 8,
	}
	varTable := map[string]string{
		ExampleStrVar.name:    strValue,
		ExampleStructVar.name: structStr,
	}
	if err := InitializeGlobalVars(varTable); err != nil {
		t.Fatal("failed to initialize global variables")
	}
	strResult, hasValue := ExampleStrVar.Value()
	if !hasValue || strResult != strValue {
		t.Errorf("ExampleStrVar.Value() returns (%v, %v); want (%v, true)", strResult, hasValue, strValue)
	}
	structResult, hasValue := ExampleStructVar.Value()
	diff := cmp.Diff(structResult, &structValue)
	if !hasValue || diff != "" {
		t.Errorf("ExampleStructVar.Value() returns (%v, %v); want (%v, true)", *structResult, hasValue, structValue)
	}
	_, hasValue = ExampleBoolVar.Value()
	if hasValue {
		t.Error("ExampleBoolVar.Value() returns true for hasValue; want false")

	}
	// 	TestInitializeGlobalVarsOnce makes sure InitializeGlobalVar allow only one call.
	if err := InitializeGlobalVars(map[string]string{}); err == nil {
		t.Error("InitializeGlobalVars returns no error on the second call")
	}
}

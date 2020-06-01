// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package protoutil

import (
	"reflect"
	"testing"

	"chromiumos/tast/services/cros/network"
)

func TestShillValMapConvert(t *testing.T) {
	// test EncodeToShillValMap and DecodeFromShillValMap
	testcases := []struct {
		normalMap   map[string]interface{}
		shillValMap ShillValMap
		shouldFail  bool
	}{
		{ // all supported types
			normalMap: map[string]interface{}{
				"bool":     true,
				"string":   "abc",
				"[]string": []string{"abc", "123"},
			},
			shillValMap: ShillValMap{
				"bool":     &network.ShillVal{Val: &network.ShillVal_Bool{Bool: true}},
				"string":   &network.ShillVal{Val: &network.ShillVal_Str{Str: "abc"}},
				"[]string": &network.ShillVal{Val: &network.ShillVal_StrArray{StrArray: &network.StrArray{Vals: []string{"abc", "123"}}}},
			},
			shouldFail: false,
		},
		{ // nil map should be also good
			normalMap:   nil,
			shillValMap: ShillValMap{},
			shouldFail:  false,
		},
		{
			normalMap: map[string]interface{}{
				"bool":   true,
				"string": "abc",
				"int":    1,
			},
			shillValMap: nil,
			shouldFail:  true, // int is not supported
		},
		{
			normalMap: map[string]interface{}{
				"[]int": []int{},
			},
			shillValMap: nil,
			shouldFail:  true, // []int is not supported
		},
		{
			normalMap: map[string]interface{}{
				"[]bool": []bool{},
			},
			shillValMap: nil,
			shouldFail:  true, // []bool is not supported
		},
	}

	for i, tc := range testcases {
		shillValMap, err := EncodeToShillValMap(tc.normalMap)
		if tc.shouldFail {
			if err == nil {
				t.Errorf("testcase %d EncodeToShillValMap should not convert successfully", i)
			}
			continue
		}
		if err != nil {
			t.Errorf("testcase %d EncodeToShillValMap failed with err=%s", i, err.Error())
			continue
		}
		if !reflect.DeepEqual(shillValMap, tc.shillValMap) {
			t.Errorf("testcase %d EncodeToShillValMap got %v but expect %v", i, shillValMap, tc.shillValMap)
			continue
		}
		normalMap, err := DecodeFromShillValMap(shillValMap)
		if err != nil {
			t.Errorf("testcase %d DecodeFromShillValMap failed with err=%s", i, err.Error())
			continue
		}
		// It is ok that the original map is nil and after two conversions it becomes empty map.
		if !(tc.normalMap == nil && len(normalMap) == 0) && !reflect.DeepEqual(normalMap, tc.normalMap) {
			t.Errorf("testcase %d DecodeFromShillValMap got %v but expect %v", i, normalMap, tc.normalMap)
			continue
		}
	}
}

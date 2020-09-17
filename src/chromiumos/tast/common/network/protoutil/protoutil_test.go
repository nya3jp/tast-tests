// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package protoutil

import (
	"reflect"
	"testing"

	"chromiumos/tast/common/shillconst"
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
				"uint32":   uint32(100),
				"[]uint32": []uint32{100, 200, 300},
			},
			shillValMap: ShillValMap{
				"bool":     &network.ShillVal{Val: &network.ShillVal_Bool{Bool: true}},
				"string":   &network.ShillVal{Val: &network.ShillVal_Str{Str: "abc"}},
				"[]string": &network.ShillVal{Val: &network.ShillVal_StrArray{StrArray: &network.StrArray{Vals: []string{"abc", "123"}}}},
				"uint32":   &network.ShillVal{Val: &network.ShillVal_Uint32{Uint32: uint32(100)}},
				"[]uint32": &network.ShillVal{Val: &network.ShillVal_Uint32Array{Uint32Array: &network.Uint32Array{Vals: []uint32{100, 200, 300}}}},
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

func TestShillPropertyChangedSignalListConvert(t *testing.T) {
	// test EncodeToShillPropertyChangedSignalList and DecodeFromShillPropertyChangedSignalList
	testcases := []struct {
		normalShillPropertyHolderList  []ShillPropertyHolder
		shillPropertyChangedSignalList ShillPropertyChangedSignalList
		shouldFail                     bool
	}{
		{
			normalShillPropertyHolderList: []ShillPropertyHolder{
				ShillPropertyHolder{
					Name:  shillconst.ServicePropertyIsConnected,
					Value: true,
				},
				ShillPropertyHolder{
					Name:  shillconst.ServicePropertyState,
					Value: "online",
				},
			},
			shillPropertyChangedSignalList: ShillPropertyChangedSignalList{
				&network.ShillPropertyChangedSignal{Prop: shillconst.ServicePropertyIsConnected, Val: &network.ShillVal{Val: &network.ShillVal_Bool{Bool: true}}},
				&network.ShillPropertyChangedSignal{Prop: shillconst.ServicePropertyState, Val: &network.ShillVal{Val: &network.ShillVal_Str{Str: "online"}}},
			},
			shouldFail: false,
		},
	}

	for i, tc := range testcases {
		shillPropertyChangedSignalList, err := EncodeToShillPropertyChangedSignalList(tc.normalShillPropertyHolderList)
		if tc.shouldFail {
			if err == nil {
				t.Errorf("testcase %d EncodeToPropertyChangedSignalList should not convert successfully", i)
			}
			continue
		}
		if err != nil {
			t.Errorf("testcase %d EncodeToPropertyChangedSignalList failed with err=%s", i, err.Error())
			continue
		}
		if !reflect.DeepEqual(shillPropertyChangedSignalList, tc.shillPropertyChangedSignalList) {
			t.Errorf("testcase %d EncodeToPropertyChangedSignalList got %v but expect %v", i, shillPropertyChangedSignalList, tc.shillPropertyChangedSignalList)
			continue
		}
		normalShillPropertyHolderList, err := DecodeFromShillPropertyChangedSignalList(shillPropertyChangedSignalList)
		if err != nil {
			t.Errorf("testcase %d DecodeFromPropertyChangedSignalList failed with err=%s", i, err.Error())
			continue
		}
		if !(tc.normalShillPropertyHolderList == nil && len(normalShillPropertyHolderList) == 0) && !reflect.DeepEqual(normalShillPropertyHolderList, tc.normalShillPropertyHolderList) {
			t.Errorf("testcase %d DecodeFromPropertyChangedSignalList got %v but expect %v", i, normalShillPropertyHolderList, tc.normalShillPropertyHolderList)
			continue
		}
	}
}

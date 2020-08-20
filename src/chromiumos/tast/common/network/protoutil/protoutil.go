// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package protoutil provides utils to deal with protobuf.
package protoutil

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/services/cros/network"
)

// ShillValMap is a type alias of map[string]*network.ShillVal. It can be sent through protobuf.
type ShillValMap map[string]*network.ShillVal

// EncodeToShillValMap converts a map that contains supported value type to protocol buffer network.ShillVal.
func EncodeToShillValMap(conf map[string]interface{}) (ShillValMap, error) {
	ret := make(ShillValMap)
	for k, v := range conf {
		val, err := ToShillVal(v)
		if err != nil {
			return nil, err
		}
		ret[k] = val
	}
	return ret, nil
}

// ToShillVal converts a common golang type to a ShillVal .
func ToShillVal(i interface{}) (*network.ShillVal, error) {
	switch x := i.(type) {
	case string:
		return &network.ShillVal{
			Val: &network.ShillVal_Str{
				Str: x,
			},
		}, nil
	case bool:
		return &network.ShillVal{
			Val: &network.ShillVal_Bool{
				Bool: x,
			},
		}, nil
	case uint32:
		return &network.ShillVal{
			Val: &network.ShillVal_Uint32{
				Uint32: x,
			},
		}, nil
	case []string:
		return &network.ShillVal{
			Val: &network.ShillVal_StrArray{
				StrArray: &network.StrArray{Vals: x},
			},
		}, nil
	default:
		return nil, errors.Errorf("unsupported type %T", x)
	}
}

// DecodeFromShillValMap converts a ShillValMap to a (key, value) map.
func DecodeFromShillValMap(conf ShillValMap) (map[string]interface{}, error) {
	ret := make(map[string]interface{})
	for k, v := range conf {
		i, err := FromShillVal(v)
		if err != nil {
			return nil, err
		}
		ret[k] = i
	}
	return ret, nil
}

// FromShillVal converts a ShillVal to a common golang type.
func FromShillVal(v *network.ShillVal) (interface{}, error) {
	switch x := v.Val.(type) {
	case *network.ShillVal_Str:
		return x.Str, nil
	case *network.ShillVal_Bool:
		return x.Bool, nil
	case *network.ShillVal_Uint32:
		return x.Uint32, nil
	case *network.ShillVal_StrArray:
		return x.StrArray.Vals, nil
	default:
		return nil, errors.Errorf("unsupported type %T", x)
	}
}

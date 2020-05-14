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
		switch x := v.(type) {
		case string:
			ret[k] = &network.ShillVal{
				Val: &network.ShillVal_Str{
					Str: x,
				},
			}
		case bool:
			ret[k] = &network.ShillVal{
				Val: &network.ShillVal_Bool{
					Bool: x,
				},
			}
		case []string:
			ret[k] = &network.ShillVal{
				Val: &network.ShillVal_StrArray{
					StrArray: &network.StrArray{StrArray: x},
				},
			}
		default:
			return nil, errors.Errorf("unsupported type %T", x)
		}
	}
	return ret, nil
}

// DecodeFromShillValMap converts a ShillValMap to a (key, value) map.
func DecodeFromShillValMap(conf ShillValMap) (map[string]interface{}, error) {
	ret := make(map[string]interface{})
	for k, v := range conf {
		switch x := v.Val.(type) {
		case *network.ShillVal_Str:
			ret[k] = x.Str
		case *network.ShillVal_Bool:
			ret[k] = x.Bool
		case *network.ShillVal_StrArray:
			ret[k] = x.StrArray.StrArray
		default:
			return nil, errors.Errorf("unsupported type %T", x)
		}
	}
	return ret, nil
}

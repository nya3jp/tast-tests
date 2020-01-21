// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package secconf

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/services/cros/network"
)

// EncodeToShillValMap converts map that contains supported value type to protocol buffer network.ShillVal.
func EncodeToShillValMap(conf map[string]interface{}) (map[string]*network.ShillVal, error) {
	ret := make(map[string]*network.ShillVal)
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
		default:
			return nil, errors.Errorf("unsupported type %T", x)
		}
	}
	return ret, nil
}

// DecodeFromShillValMap converts map that contains protocol buffer network.ShillVal to supported value type.
func DecodeFromShillValMap(conf map[string]*network.ShillVal) (map[string]interface{}, error) {
	ret := make(map[string]interface{})
	for k, v := range conf {
		switch x := v.Val.(type) {
		case *network.ShillVal_Str:
			ret[k] = x.Str
		case *network.ShillVal_Bool:
			ret[k] = x.Bool
		default:
			return nil, errors.Errorf("unsupported type %T", x)
		}
	}
	return ret, nil
}

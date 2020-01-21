// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package secconf

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/services/cros/network"
)

// MapToPbMap convert map with normal type keys to protocol buffers type keys.
func MapToPbMap(conf map[string]interface{}) (map[string]*network.ShillVal, error) {
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
			return nil, errors.Errorf("unknown type %T, cannot convert to protocol buffers type", x)
		}
	}
	return ret, nil
}

// PbMapToMap convert map with protocol buffers type keys to normal type keys.
func PbMapToMap(conf map[string]*network.ShillVal) (map[string]interface{}, error) {
	ret := make(map[string]interface{})
	for k, v := range conf {
		switch x := v.Val.(type) {
		case *network.ShillVal_Str:
			ret[k] = x.Str
		case *network.ShillVal_Bool:
			ret[k] = x.Bool
		default:
			return nil, errors.Errorf("unknown type %T, cannot convert to normal type", x)
		}
	}
	return ret, nil
}

// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
)

// NetworkTypeFromShillType returns an analogous NetworkType.
func NetworkTypeFromShillType(sType string) (NetworkType, error) {
	if sType == shillconst.TypeEthernet {
		return EthernetNT, nil
	} else if sType == shillconst.TypeWifi {
		return WiFiNT, nil
	} else if sType == shillconst.TypeCellular {
		return CellularNT, nil
	} else if sType == shillconst.TypeVPN {
		return VPNNT, nil
	}

	return EthernetNT, errors.New("unknown shill type")
}

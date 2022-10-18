// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quicksettings

import (
	"chromiumos/tast/services/cros/ui"
)

// BluetoothDetailedView is the root of the Bluetooth detailed view within Quick Settings.
var BluetoothDetailedView = &ui.Finder{
	NodeWiths: []*ui.NodeWith{
		{Value: &ui.NodeWith_HasClass{HasClass: "BluetoothDetailedViewImpl"}},
	},
}

// PairNewDeviceButton is the "Pair new device" button within the Bluetooth detailed view.
var PairNewDeviceButton = &ui.Finder{
	NodeWiths: []*ui.NodeWith{
		{Value: &ui.NodeWith_NameContaining{NameContaining: "Pair new device"}},
		{Value: &ui.NodeWith_HasClass{HasClass: "IconButton"}},
		{Value: &ui.NodeWith_Ancestor{Ancestor: BluetoothDetailedView}},
	},
}

// PairNewDeviceDialog is the dialog opened when "Pair new device" is clicked within the Bluetooth detailed view.
var PairNewDeviceDialog = &ui.Finder{
	NodeWiths: []*ui.NodeWith{
		{Value: &ui.NodeWith_NameContaining{NameContaining: "Pair new device"}},
		{Value: &ui.NodeWith_Role{Role: ui.Role_ROLE_ROOT_WEB_AREA}},
	},
}

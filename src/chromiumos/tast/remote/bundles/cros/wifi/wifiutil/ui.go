// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"chromiumos/tast/services/cros/ui"
)

// JoinWiFiNetworkDialogFinder is the dialog opened when connecting to a secured WiFi network.
var JoinWiFiNetworkDialogFinder = &ui.Finder{
	NodeWiths: []*ui.NodeWith{
		{Value: &ui.NodeWith_NameContaining{NameContaining: "Join Wi-Fi network"}},
		{Value: &ui.NodeWith_Role{Role: ui.Role_ROLE_DIALOG}},
	},
}

// PasswordFieldFinder is the password field within the "Join Wi-Fi network" dialog.
var PasswordFieldFinder = &ui.Finder{
	NodeWiths: []*ui.NodeWith{
		{Value: &ui.NodeWith_NameContaining{NameContaining: "Password"}},
		{Value: &ui.NodeWith_Role{Role: ui.Role_ROLE_TEXT_FIELD}},
		{Value: &ui.NodeWith_Ancestor{Ancestor: JoinWiFiNetworkDialogFinder}},
	},
}

// ConnectButtonFinder is the connect button within the "Join Wi-Fi network" dialog.
var ConnectButtonFinder = &ui.Finder{
	NodeWiths: []*ui.NodeWith{
		{Value: &ui.NodeWith_NameContaining{NameContaining: "Connect"}},
		{Value: &ui.NodeWith_Role{Role: ui.Role_ROLE_BUTTON}},
		{Value: &ui.NodeWith_Ancestor{Ancestor: JoinWiFiNetworkDialogFinder}},
	},
}

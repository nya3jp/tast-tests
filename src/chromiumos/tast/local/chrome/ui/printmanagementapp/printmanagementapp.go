// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package printmanagementapp contains common functions used in the app.
package printmanagementapp

import (
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ui"
)

var printManagementRootNodeParams = ui.FindParams{
	Name: apps.PrintManagement.Name,
	Role: ui.RoleTypeWindow,
}

var printManagementClearHistoryButton = ui.FindParams{
	Name: "Clear all history",
	Role: ui.RoleTypeButton,
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import "chromiumos/tast/local/chrome/ui"

// OsSettingsURLPrefix is the prefix URL to all Chrome OS settings pages.
const OsSettingsURLPrefix = "chrome://os-settings/"

// NearbySettingsSubPageURL is the URL to Nearby Settings subpage.
const NearbySettingsSubPageURL = "multidevice/nearbyshare"

// NearbySettingsUIParams matches "pageTitle" in NearbySettings sub page.
var NearbySettingsUIParams ui.FindParams = ui.FindParams{
	Role: ui.RoleTypeHeading,
	Name: "Nearby Share",
}

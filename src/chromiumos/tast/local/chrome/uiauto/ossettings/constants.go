// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ossettings

import (
	"fmt"
	"regexp"

	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// WindowFinder is the finder for the Settings window.
var WindowFinder *nodewith.Finder = nodewith.MultilingualNameStartingWith("Settings", map[string]string{"de": "Einstellungen"}).
	Role(role.Window).First()

// SearchBoxFinder is the finder for the search box in the settings app.
var SearchBoxFinder = nodewith.MultilingualName("Search settings", map[string]string{"de": "In Einstellungen suchen"}).
	Role(role.SearchBox).Ancestor(WindowFinder)

// Advanced is a button to expand advanced options.
var Advanced = nodewith.Role(role.Button).Ancestor(nodewith.Role(role.Heading).Name("Advanced"))

// Network is a subpage link.
var Network = nodewith.Name("Network").Role(role.Link).HasClass("item").Focusable()

// Bluetooth is a subpage link.
var Bluetooth = nodewith.Name("Bluetooth").Role(role.Link).HasClass("item").Focusable()

// ConnectedDevices is a subpage link.
var ConnectedDevices = nodewith.Name("Connected devices").Role(role.Link).HasClass("item").Focusable()

// Accounts is a subpage link.
var Accounts = nodewith.Name("Accounts").Role(role.Link).HasClass("item").Focusable()

// Device is a subpage link.
var Device = nodewith.Name("Device").Role(role.Link).HasClass("item").Focusable()

// Personalization is a subpage link.
var Personalization = nodewith.Name("Personalization").Role(role.Link).HasClass("item").Focusable()

// SearchAndAssistant is a subpage link.
var SearchAndAssistant = nodewith.Name("Search and Assistant").Role(role.Link).HasClass("item").Focusable()

// SecurityAndPrivacy is a subpage link.
var SecurityAndPrivacy = nodewith.Name("Security and Privacy").Role(role.Link).HasClass("item").Focusable()

// Apps is a subpage link.
var Apps = nodewith.Name("Apps").Role(role.Link).HasClass("item").Focusable()

// DateAndTime is a subpage link.
var DateAndTime = nodewith.Name("Date and time").Role(role.Link).HasClass("item").Focusable()

// LanguagesAndInputs is a subpage link.
var LanguagesAndInputs = nodewith.Name("Languages and inputs").Role(role.Link).HasClass("item").Focusable()

// Files is a subpage link.
var Files = nodewith.Name("Files").Role(role.Link).HasClass("item").Focusable()

// PrintAndScan is a subpage link.
var PrintAndScan = nodewith.Name("Print and scan").Role(role.Link).HasClass("item").Focusable()

// Developers is a subpage link.
var Developers = nodewith.Name("Developers").Role(role.Link).HasClass("item").Focusable()

// Accessibility is a subpage link.
var Accessibility = nodewith.Name("Accessibility").Role(role.Link).HasClass("item").Focusable()

// ResetSettings is a subpage link.
var ResetSettings = nodewith.Name("Reset settings").Role(role.Link).HasClass("item").Focusable()

// AboutChromeOS is a subpage link.
var AboutChromeOS = nodewith.MultilingualName("About Chrome OS", map[string]string{"de": "Über Chrome OS"}).
	Role(role.Link)

// searchMismatched is the pattern shown in search results
// when the input keyword in `SearchBox` is mismatched with any existing option.
var searchMismatched = `No search results found`

// searchResultFinder is a finder of all possible search results.
var searchResultFinder = nodewith.NameRegex(regexp.MustCompile(fmt.Sprintf(`(Search result \d+ of \d+: .*|%s)`, searchMismatched))).Onscreen()

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

// Elements in "AboutChromeOS"
var (
	VersionInfo       = nodewith.NameStartingWith("Version ").Role(role.StaticText)
	CheckUpdateBtn    = nodewith.Name("Check for updates").Role(role.Button)
	ReportIssue       = nodewith.Name("Report an issue").Role(role.Link)
	AdditionalDetails = nodewith.Name("Additional details").Role(role.Link)
	TermsOfService    = nodewith.Name("Terms of Service").Role(role.Link)

	// OpenSourceSoftwares matches two links, needs specify when using.
	OpenSourceSoftwares = nodewith.Name("open source software").Role(role.Link)

	ChangeChannelBtn = nodewith.Name("Change channel").Role(role.Button)
	BuildDetailsBtn  = nodewith.Name("Build details").Role(role.Button)
)

// BackArrowBtn is the button to return to last page.
var BackArrowBtn = nodewith.HasClass("icon-arrow-back").Role(role.Button)

// searchMismatched is the pattern shown in search results
// when the input keyword in `SearchBox` is mismatched with any existing option.
var searchMismatched = `No search results found`

// searchResultFinder is a finder of all possible search results.
var searchResultFinder = nodewith.NameRegex(regexp.MustCompile(fmt.Sprintf(`(Search result \d+ of \d+: .*|%s)`, searchMismatched))).Onscreen()

// networkFinder is the finder for the Network page UI in OS setting.
var networkFinder = nodewith.Name("Network").Role(role.Link).Ancestor(WindowFinder)

// mobileButton is the finder for the Mobile Data page button UI in network page.
var mobileButton = nodewith.Name("Mobile data").Role(role.Button)

// AddCellularButton is the finder for the Add Cellular button in cellular network list
var AddCellularButton = nodewith.NameStartingWith("Add Cellular").Role(role.Button)

// Elements in "Cellular detail page"
var (
	// connectedStatus is the finder for the connected status text UI in the cellular detail page.
	ConnectedStatus = nodewith.Name("Connected").Role(role.StaticText)

	// disconnectedStatus is the finder for the disconnected status text UI in the cellular detail page.
	DisconnectedStatus = nodewith.Name("Not Connected").Role(role.StaticText)

	// connectButton is the finder for the connect button UI in the cellular detail page.
	ConnectButton = nodewith.Name("Connect").Role(role.Button)

	// disconnectButton is the finder for the disconnect button UI in the cellular detail page.
	DisconnectButton = nodewith.Name("Disconnect").Role(role.Button)

	// RoamingToggle is the finder for the roaming toggle UI in the cellular detail page.
	RoamingToggle = nodewith.Name("Allow mobile data roaming").Role(role.ToggleButton)
)

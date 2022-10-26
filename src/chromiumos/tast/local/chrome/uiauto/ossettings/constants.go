// Copyright 2021 The ChromiumOS Authors
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

// Elements in "Languages page"
var (
	// AddLanguagesButton is the finder for the add language button UI in the languages page.
	AddLanguagesButton = nodewith.Name("Add languages").Role(role.Button)

	// SearchLanguages is the finder for the search language searchbox UI in the languages page.
	SearchLanguages = nodewith.Name("Search languages").Role(role.SearchBox)
)

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
var AboutChromeOS = nodewith.MultilingualName("About ChromeOS", map[string]string{"de": "Über ChromeOS"}).
	Role(role.Link)

// MenuButton is a button to show the menu on the left side, only exist when the menu does not exist.
var MenuButton = nodewith.Name("Main menu").Role(role.Button).Focusable()

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

// NextButton is the finder for a button labelled as Next.
var NextButton = nodewith.NameContaining("Next").Role(role.Button)

// DoneButton is the finder for a button labelled as Done.
var DoneButton = nodewith.NameContaining("Done").Role(role.Button)

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
	// ConnectedStatus is the finder for the connected status text UI in the cellular detail page.
	ConnectedStatus = nodewith.Name("Connected").Role(role.StaticText)

	// DisconnectedStatus is the finder for the disconnected status text UI in the cellular detail page.
	DisconnectedStatus = nodewith.Name("Not Connected").Role(role.StaticText)

	// ConnectingStatus is the finder for the connecting status text UI in the cellular detail page.
	ConnectingStatus = nodewith.NameStartingWith("Connecting").Role(role.StaticText)

	// connectButton is the finder for the connect button UI in the cellular detail page.
	ConnectButton = nodewith.Name("Connect").Role(role.Button)

	// disconnectButton is the finder for the disconnect button UI in the cellular detail page.
	DisconnectButton = nodewith.Name("Disconnect").Role(role.Button)

	// RoamingToggle is the finder for the roaming toggle UI in the cellular detail page.
	RoamingToggle = nodewith.Name("Allow mobile data roaming").Role(role.ToggleButton)

	// CellularAdvanced is the finder for the button that collpases/expands the advanced section of cellular details page.
	CellularAdvanced = nodewith.Name("Show advanced network properties").Role(role.Button)

	// LockSimToggle is the finder for the Lock SIM toggle UI in the cellular details page.
	LockSimToggle = nodewith.NameStartingWith("Lock").Role(role.ToggleButton)

	// EnterButton is the finder for the Enter button in the SIM lock dialog UI.
	EnterButton = nodewith.Role(role.Button).Name("Enter")

	// VisibilityButton is the finder for the show PIN button in the SIM lock dialog UI.
	VisibilityButton = nodewith.Role(role.Button).HasClass("icon-visibility")
)

// Elements in "Proxy" section of Network page.
var (
	// ShowProxySettingsTab is the finder for the "show proxy settings" tab.
	ShowProxySettingsTab = nodewith.HasClass("settings-box").Name("Show proxy settings").Role(role.GenericContainer)

	// SharedNetworksToggleButton is the finder for the "show shared networks" button.
	SharedNetworksToggleButton = nodewith.Name("Allow proxies for shared networks").Role(role.ToggleButton)

	// ConfirmButton is the finder for the "confirm" button.
	ConfirmButton = nodewith.Name("Confirm").Role(role.Button)

	proxyDropDownNameRegex = regexp.MustCompile(`(C|c)onnection type`)
	// ProxyDropDownMenu is the finder for the proxy drop down menu.
	ProxyDropDownMenu = nodewith.HasClass("md-select").NameRegex(proxyDropDownNameRegex).Role(role.ComboBoxSelect)

	// ManualProxyOption is the finder for the "Manual proxy configuration" option in the proxy drop down menu.
	ManualProxyOption = nodewith.Name("Manual proxy configuration").Role(role.ListBoxOption)

	// HTTPHostTextField is the finder for the "HTTP host" text field.
	HTTPHostTextField = nodewith.Name("HTTP Proxy - Host").Role(role.TextField)

	// HTTPPortTextField is the finder for the "HTTP port" text field.
	HTTPPortTextField = nodewith.Name("HTTP Proxy - Port").Role(role.TextField)

	// HTTPSHostTextField is the finder for the "Secure HTTP host" text field.
	HTTPSHostTextField = nodewith.Name("Secure HTTP Proxy - Host").Role(role.TextField)

	// HTTPSPortTextField is the finder for the "Secure HTTP port" text field.
	HTTPSPortTextField = nodewith.Name("Secure HTTP Proxy - Port").Role(role.TextField)

	// SocksHostTextField is the finder for the "SOCKS host" text field.
	SocksHostTextField = nodewith.Name("SOCKS Host - Host").Role(role.TextField)

	// SocksPortTextField is the finder for the "SOCKS port" text field.
	SocksPortTextField = nodewith.Name("SOCKS Host - Port").Role(role.TextField)
)

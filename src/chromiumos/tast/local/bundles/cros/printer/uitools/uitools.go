// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package uitools contains strings and different nodewith.Finder objects from
// the UI that can be used by multiple tests.
package uitools

import (
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// List of strings that are used in the UI.
const (
	AddPrinterManuallyName = "Add a printer manually"
	AddPrinterName         = "Add printer"
	AddName                = "Add"
	AddressName            = "Address"
	AdvancedConfigName     = "Advanced printer configuration"
	AppSocketName          = "AppSocket"
	CreditsName            = "Credits"
	EditName               = "Edit"
	EditPrinterName        = "Edit printer"
	EulaName               = "End User License Agreement"
	ManufacturerName       = "Manufacturer"
	ModelName              = "Model"
	NameName               = "Name"
	PpdHeaderName          = "PPD for "
	PpdStartTextName       = "PPD-Adobe:"
	PpdRetrieveErrorName   = "Unable to retrieve PPD"
	PrintersName           = "Printers"
	ProtocolName           = "Protocol"
	SettingsPageName       = "osPrinting"
	ViewPpdName            = "View PPD"
)

// nodewith.Finder objects for items in the UI.
var (
	AddPrinterManuallyFinder *nodewith.Finder = nodewith.Role(role.Dialog).Name(AddPrinterManuallyName)
	AddPrinterFinder         *nodewith.Finder = nodewith.Role(role.Button).Name(AddPrinterName)
	AddFinder                *nodewith.Finder = nodewith.Role(role.Button).Name(AddName)
	AddressFinder            *nodewith.Finder = nodewith.Role(role.TextField).Name(AddressName)
	AdvancedConfigFinder     *nodewith.Finder = nodewith.Role(role.Dialog).Name(AdvancedConfigName)
	AppSocketFinder          *nodewith.Finder = nodewith.Role(role.ListBoxOption).NameContaining(AppSocketName)
	CreditsFinder            *nodewith.Finder = nodewith.Role(role.RootWebArea).Name(CreditsName)
	EditFinder               *nodewith.Finder = nodewith.Role(role.StaticText).Name(EditName)
	EditPrinterFinder        *nodewith.Finder = nodewith.Role(role.Dialog).Name(EditPrinterName)
	EulaFinder               *nodewith.Finder = nodewith.Role(role.Link).Name(EulaName)
	ManufacturerFinder       *nodewith.Finder = nodewith.Role(role.TextField).Name(ManufacturerName)
	ModelFinder              *nodewith.Finder = nodewith.Role(role.TextField).Name(ModelName)
	NameFinder               *nodewith.Finder = nodewith.Role(role.TextField).Name(NameName)
	PpdStartTextFinder       *nodewith.Finder = nodewith.Role(role.StaticText).NameContaining(PpdStartTextName)
	PpdRetrieveErrorFinder   *nodewith.Finder = nodewith.Role(role.StaticText).NameContaining(PpdRetrieveErrorName)
	PrintersFinder           *nodewith.Finder = nodewith.Role(role.Link).Name(PrintersName)
	ProtocolFinder           *nodewith.Finder = nodewith.Role(role.ComboBoxSelect).Name(ProtocolName)
	ViewPpdFinder            *nodewith.Finder = nodewith.Role(role.Button).Name(ViewPpdName)
)

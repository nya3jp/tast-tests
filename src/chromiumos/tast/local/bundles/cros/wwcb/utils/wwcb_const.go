// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"time"

	"chromiumos/tast/testing"
)

// WWCBServerURL using in var
const (
	WWCBServerURL = "WWCBIP"
	PerpUsb       = "PerpUsb"
)

// InputArguments command line args
var InputArguments = []string{
	WWCBServerURL,
	"Display.HDMI_Switch",
	"Display.TYPEA_Switch",
	"Display.TYPEC_Switch",
	"Display.VGA_Switch",
	"Display.DP_Switch",
	"Display.DVI_Switch",
	"Display.ETHERNET_Switch",
	"Station.HDMI_Switch",
	"Station.TYPEA_Switch",
	"Station.TYPEC_Switch",
	"Station.VGA_Switch",
	"Station.DP_Switch",
	"Station.DVI_Switch",
	"Station.ETHERNET_Switch",
	"PerpUsb.HDMI_Switch",
	"PerpUsb.TYPEA_Switch",
	"PerpUsb.TYPEC_Switch",
	"PerpUsb.VGA_Switch",
	"PerpUsb.DP_Switch",
	"PerpUsb.DVI_Switch",
	"PerpUsb.ETHERNET_Switch",
	"Other.HDMI_Switch",
	"Other.TYPEA_Switch",
	"Other.TYPEC_Switch",
	"Other.VGA_Switch",
	"Other.DP_Switch",
	"Other.DVI_Switch",
	"Other.ETHERNET_Switch",
}

// DefaultOSSettingsPollOptions variables for scripts
var DefaultOSSettingsPollOptions = &testing.PollOptions{
	Timeout:  10 * time.Second,
	Interval: 1 * time.Second,
}

// list switch types
const (
	SwitchETHERNET = "ETHERNET_Switch"
	SwitchTYPEA    = "TYPEA_Switch"
	SwitchTYPEC    = "TYPEC_Switch"
	SwitchHDMI     = "HDMI_Switch"
	SwitchVGA      = "VGA_Switch"
	SwitchDVI      = "DVI_Switch"
	SwitchDP       = "DP_Switch"
)

// station
const (
	StationType  = "Docking_TYPEC_Switch"
	StationIndex = "ID1"
)

// display
const (

	// build in display
	// type is blank
	IntDispType  = ""
	IntDispIndex = "ID1"

	// first external display using hdmi
	ExtDisp1Type  = "Display_HDMI_Switch"
	ExtDisp1Index = "ID2"

	// second external display using dp
	ExtDisp2Type  = "Display_DP_Switch"
	ExtDisp2Index = "ID2"
)

// ethernet
const (
	EthernetType  = "ETHERNET_Switch"
	EthernetIndex = "ID1"
)

// usb print
const (
	USBPrinterType  = "TYPEA_Switch"
	USBPrinterIndex = "ID1"
)

// input vars
const (
	Docking  = "Docking"
	ExtDisp1 = "External.Display.1"
	ExtDisp2 = "External.Display.2"
	USBPerp  = "USBPerp"
)

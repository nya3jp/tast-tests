// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package svcstate defines the values of Service's state in shill.
package svcstate

// Service state values defined in dbus-constants.h
const (
	Idle              = "idle"
	Carrier           = "carrier"
	Association       = "association"
	Configuration     = "configuration"
	Ready             = "ready"
	Portal            = "portal"
	NoConnectivity    = "no-connectivity"
	RedirectFound     = "redirect-found"
	PortalSuspected   = "portal-suspected"
	Offline           = "offline"
	Online            = "online"
	Disconnect        = "disconnecting"
	Failure           = "failure"
	ActivationFailure = "activation-failure"
)

// ConnectedStates is a list of service states that are considered connected.
var ConnectedStates = []string{
	Portal,
	NoConnectivity,
	RedirectFound,
	PortalSuspected,
	Online,
	Ready,
}

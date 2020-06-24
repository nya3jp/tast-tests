// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package profileprop defines the constant keys of Profile's properties in shill.
package profileprop

// Profile property names.
const (
	CheckPortalList           = "CheckPortalList"
	Entries                   = "Entries"
	Name                      = "Name"
	PortalURL                 = "PortalURL"
	PortalCheckInterval       = "PortalCheckInterval"
	Services                  = "Services"
	UserHash                  = "UserHash"
	ProhibitedTechnologies    = "ProhibitedTechnologies"
	ArpGateway                = "ArpGateway"
	LinkMonitorTechnologies   = "LinkMonitorTechnologies"
	NoAutoConnectTechnologies = "NoAutoConnectTechnologies"
)

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package request

import (
	"github.com/google/uuid"
)

// OS is part of an Omaha Request.
type OS struct {
	XMLName struct{} `xml:"os"`

	Version  string `xml:"version,attr"`
	Platform string `xml:"platform,attr"`
	SP       string `xml:"sp,attr"`
}

// RequestUpdateCheck is part of an Omaha Request.
type RequestUpdateCheck struct { // nolint
	XMLName struct{} `xml:"updatecheck"`

	TargetVersionPrefix string `xml:"targetversionprefix,omitempty,attr"`
	RollbackAllowed     bool   `xml:"rollback_allowed,attr,omitempty"`
	LTSTag              string `xml:"ltstag,omitempty,attr"`
}

// Ping is part of an Omaha Request.
type Ping struct {
	XMLName struct{} `xml:"ping"`

	R int `xml:"r"`
}

// RequestApp is part of an Omaha Request.
type RequestApp struct { // nolint
	XMLName struct{} `xml:"app"`

	APPID         string `xml:"appid,attr"`
	Cohort        string `xml:"cohort,attr,omitempty"`
	CohortName    string `xml:"cohortname,attr,omitempty"`
	CohortHint    string `xml:"cohorthint,attr,omitempty"`
	Version       string `xml:"version,attr,omitempty"`
	FromVersion   string `xml:"from_version,attr,omitempty"`
	Track         string `xml:"track,attr,omitempty"`
	FromTrack     string `xml:"from_track,attr,omitempty"`
	Fingerprint   string `xml:"fingerprint,attr,omitempty"`
	OSBuildType   string `xml:"os_build_type,attr,omitempty"`
	InstallDate   int    `xml:"installdate,attr,omitempty"`
	Board         string `xml:"board,attr,omitempty"`
	HardwareClass string `xml:"hardware_class,attr,omitempty"`
	DeltaOk       bool   `xml:"delta_okay,attr,omitempty"`

	UpdateCheck RequestUpdateCheck `xml:"updatecheck"`
	Lang        string             `xml:"lang,attr,omitempty"`
	Requisition string             `xml:"requisition,attr,omitempty"`

	Ping *Ping `xml:"ping,omitempty"`
}

// Request can be marshaled into an Omaha request, all required fields should be available.
// Structure was created based on update_engine implementation.
type Request struct {
	XMLName struct{} `xml:"request"`

	RequestID      uuid.UUID `xml:"requestid,attr"`
	SessionID      uuid.UUID `xml:"sessionid,attr"`
	Protocol       string    `xml:"protocol,attr"`
	Updater        string    `xml:"updater,attr"`
	UpdaterVersion string    `xml:"updaterversion,attr"`
	InstallSource  string    `xml:"installsource,attr"`
	IsMachine      int       `xml:"ismachine,attr"`

	OS   OS           `xml:"os"`
	Apps []RequestApp `xml:"app"`
}

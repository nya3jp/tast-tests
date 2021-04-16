// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package request

import (
	"github.com/google/uuid"
)

type OS struct {
	XMLName struct{} `xml:"os"`

	Version  string `xml:"version,attr"`
	Platform string `xml:"platform,attr"`
	SP       string `xml:"sp,attr"`
}

type UpdateCheck struct {
	XMLName struct{} `xml:"updatecheck"`

	TargetVersionPrefix string `xml:"targetversionprefix,omitempty,attr"`
	RollbackAllowed     bool   `xml:"rollback_allowed,attr,omitempty"`
	LTSTag              string `xml:"ltstag,omitempty,attr"`
}

type Ping struct {
	XMLName struct{} `xml:"ping"`

	R int `xml:"r"`
}

type App struct {
	XMLName struct{} `xml:"app"`

	APPID       string `xml:"appid,attr"`
	Cohort      string `xml:"cohort,attr,omitempty"`
	CohortName  string `xml:"cohortname,attr,omitempty"`
	CohortHint  string `xml:"cohorthint,attr,omitempty"`
	Version     string `xml:"version,attr,omitempty"`
	FromVersion string `xml:"from_version,attr,omitempty"`
	Track       string `xml:"track,attr,omitempty"`
	FromTrack   string `xml:"from_track,attr,omitempty"`
	// TODO: Product components
	Fingerprint   string `xml:"fingerprint,attr,omitempty"`
	OSBuildType   string `xml:"os_build_type,attr,omitempty"`
	InstallDate   int    `xml:"installdate,attr,omitempty"`
	Board         string `xml:"board,attr,omitempty"`
	HardwareClass string `xml:"hardware_class,attr,omitempty"`
	DeltaOk       bool   `xml:"delta_okay,attr,omitempty"`

	UpdateCheck UpdateCheck `xml:"updatecheck"`
	Lang        string      `xml:"lang,attr,omitempty"`
	Requisition string      `xml:"requisition,attr,omitempty"`

	Ping *Ping `xml:"ping,omitempty"`
	// TODO: events
}

type Request struct {
	XMLName struct{} `xml:"request"`

	RequestID      uuid.UUID `xml:"requestid,attr"`
	SessionID      uuid.UUID `xml:"sessionid,attr"`
	Protocol       string    `xml:"protocol,attr"`
	Updater        string    `xml:"updater,attr"`
	UpdaterVersion string    `xml:"updaterversion,attr"`
	InstallSource  string    `xml:"installsource,attr"`
	IsMachine      int       `xml:"ismachine,attr"`

	OS   OS    `xml:"os"`
	Apps []App `xml:"app"`
}

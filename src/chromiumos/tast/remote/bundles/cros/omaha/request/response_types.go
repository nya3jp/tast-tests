// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package request

import "chromiumos/tast/errors"

// DayStart is part of an Omaha Request.
type DayStart struct {
	XMLName struct{} `xml:"daystart" json:"-"`

	ElapsedDays    int `xml:"elapsed_days,attr"`
	ElapsedSeconds int `xml:"elapsed_seconds,attr"`
}

// Action is part of an Omaha Request.
type Action struct {
	XMLName struct{} `xml:"action" json:"-"`

	Event           string `xml:"event,attr"`
	ChromeVersion   string `xml:"ChromeVersion,attr"`
	ChromeOSVersion string `xml:"ChromeOSVersion,attr"`
}

// Actions is part of an Omaha Request.
type Actions struct {
	XMLName struct{} `xml:"actions" json:"-"`

	Actions []Action `xml:"action"`
}

// Manifest is part of an Omaha Request.
type Manifest struct {
	XMLName struct{} `xml:"manifest" json:"-"`

	Version string `xml:"version,attr"`

	Actions Actions `xml:"actions"`
}

// ResponseUpdateCheck is part of an Omaha Request.
type ResponseUpdateCheck struct {
	XMLName struct{} `xml:"updatecheck" json:"-"`

	Status   string `xml:"status,attr"`
	Rollback bool   `xml:"_rollback,attr"`

	Manifest Manifest `xml:"manifest"`
}

// ResponseApp is part of an Omaha Request.
type ResponseApp struct {
	XMLName struct{} `xml:"app" json:"-"`

	AppID      string `xml:"appid,attr"`
	Cohort     string `xml:"cohort,attr,omitempty"`
	CohortName string `xml:"cohortname,attr,omitempty"`
	Status     string `xml:"status,attr"`

	UpdateCheck ResponseUpdateCheck `xml:"updatecheck"`
}

// Response can be parsed from an Omaha response.
// Structure based on real Omaha responses.
// Official documentation: https://chromium.googlesource.com/chromium/src.git/+/refs/heads/main/docs/updater/protocol_3_1.md
type Response struct {
	XMLName struct{} `xml:"response" json:"-"`

	Protocol string `xml:"protocol,attr"`
	Server   string `xml:"server,attr"`

	DayStart DayStart    `xml:"daystart"`
	App      ResponseApp `xml:"app"`
}

func (r *Response) postInstallEvent() (*Action, error) {
	for _, action := range r.App.UpdateCheck.Manifest.Actions.Actions {
		if action.Event == "postinstall" {
			return &action, nil
		}
	}

	return nil, errors.New("could not find postinstall event in response")
}

// ChromeVersion gets the Chrome version from an Omaha response.
// Returns an error if not found.
func (r *Response) ChromeVersion() (string, error) {
	action, err := r.postInstallEvent()
	if err != nil {
		return "", err
	}

	return action.ChromeVersion, nil
}

// ChromeOSVersion gets the Chrome OS version from an Omaha response.
// Returns an error if not found.
func (r *Response) ChromeOSVersion() (string, error) {
	action, err := r.postInstallEvent()
	if err != nil {
		return "", err
	}

	return action.ChromeOSVersion, nil
}

// ValidateUpdateResponse checks that the response contains an update payload.
func (r *Response) ValidateUpdateResponse() error {
	if r.Server != "prod" {
		return errors.Errorf("reached wrong server: got %q; want %q", r.Server, "prod")
	}

	if r.App.Status != "ok" {
		return errors.Errorf("unexpected App status: got %q; want %q", r.App.Status, "ok")
	}

	if r.App.UpdateCheck.Status != "ok" {
		return errors.Errorf("unexpected UpdateCheck status: got %q; want %q", r.App.UpdateCheck.Status, "ok")
	}

	if _, err := r.postInstallEvent(); err != nil {
		return err
	}

	return nil
}

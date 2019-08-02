// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Handles client (DUT) WiFi operations for wificell tests.
type WiFiClient struct {
	dut  *dut.DUT
	ssid string
}

const (
	// TODO: all of this shell garbage should leverage client-targeted
	// D-Bus libraries.
	wifiScript  = "/usr/local/autotest/cros/scripts/wifi"
	popProfiles = `dbus-send --system --print-reply --dest=org.chromium.flimflam \
	               / org.chromium.flimflam.Manager.PopAllUserProfiles`
	getProfileEntries = `dbus-send --system --print-reply --dest=org.chromium.flimflam \
	                     /profile/default org.chromium.flimflam.Profile.GetProperties | \
			     sed -nEe '/string "Entries"/,/\)/ {
			       /array \[/,/\]$/ {
			         s/^ *string "(.*)"$/\1/p
			       }
			     }'`
	getProfileEntryTypes = `dbus-send --system --print-reply --dest=org.chromium.flimflam \
	                        /profile/default org.chromium.flimflam.Profile.GetEntry string:"%s" | \
				sed -nEe '/^ {9}string "Type"$/,/\)$/ s/^ *variant *string "(.*)"$/\1/p'`
	deleteProfileEntry = `dbus-send --system --print-reply --dest=org.chromium.flimflam \
	                      /profile/default org.chromium.flimflam.Profile.DeleteEntry string:"%s"`
	createProfile = "dbus-send --system --print-reply --dest=org.chromium.flimflam / org.chromium.flimflam.Manager.CreateProfile string:wifitest"
	pushProfile   = "dbus-send --system --print-reply --dest=org.chromium.flimflam / org.chromium.flimflam.Manager.PushProfile string:wifitest"
	popProfile    = "dbus-send --system --print-reply --dest=org.chromium.flimflam / org.chromium.flimflam.Manager.PopProfile string:wifitest"
	removeProfile = "dbus-send --system --print-reply --dest=org.chromium.flimflam / org.chromium.flimflam.Manager.RemoveProfile string:wifitest"
)

func NewWiFiClient(dut *dut.DUT) *WiFiClient {
	return &WiFiClient{
		dut: dut,
	}
}

func (c *WiFiClient) Connect(ctx context.Context, ssid string) error {
	c.initProfiles(ctx)

	testing.ContextLogf(ctx, "Connecting to SSID \"%s\" from DUT", ssid)

	out, err := c.dut.Run(ctx, fmt.Sprintf("%s connect \"%s\"", wifiScript, ssid))
	testing.ContextLog(ctx, string(out))
	if err != nil {
		return errors.Wrap(err, "failed to connect")
	}
	c.ssid = ssid

	return nil
}

func (c *WiFiClient) Stop(ctx context.Context) error {
	return c.cleanupProfiles(ctx)
}

// Clear out any existing profiles and any "wifi" profile entries in the
// default profile, and set up a new profile for testing.
func (c *WiFiClient) initProfiles(ctx context.Context) error {
	if out, err := c.dut.Run(ctx, popProfiles); err != nil {
		return errors.Wrapf(err, "failed to pop profiles: %s", string(out))
	}

	out, err := c.dut.Run(ctx, getProfileEntries)
	if err != nil {
		return errors.Wrapf(err, "failed to get profile entries: %s", string(out))
	}

	entries := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, e := range entries {
		out, err = c.dut.Run(ctx, fmt.Sprintf(getProfileEntryTypes, e))
		if err != nil {
			return errors.Wrapf(err, "failed to get entry type: %s", string(out))
		}
		if t := strings.TrimSpace(string(out)); t == "wifi" {
			testing.ContextLogf(ctx, "Deleting entry %s type %s", e, t)
			if out, err = c.dut.Run(ctx, fmt.Sprintf(deleteProfileEntry, e)); err != nil {
				return errors.Wrapf(err, "failed to delete entry %s: %s", e, string(out))
			}
		}
	}

	// Ignore deletion errors. Don't ignore creation errors.
	if out, err = c.dut.Run(ctx, removeProfile+"; "+createProfile+" && "+pushProfile); err != nil {
		return errors.Wrap(err, "failed to push test profile")
	}

	return nil
}

func (c *WiFiClient) cleanupProfiles(ctx context.Context) error {
	if c.ssid != "" {
		if out, err := c.dut.Run(ctx, fmt.Sprintf("%s disconnect \"%s\"", wifiScript, c.ssid)); err != nil {
			testing.ContextLog(ctx, "failed to disconnect: ", string(out))
		}
	}

	if out, err := c.dut.Run(ctx, popProfile+"; "+removeProfile); err != nil {
		return errors.Wrapf(err, "failed to pop test profile: %s", string(out))
	}

	return nil
}

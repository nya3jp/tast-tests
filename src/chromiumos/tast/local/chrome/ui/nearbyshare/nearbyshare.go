// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbyshare is for controlling Nearby Share.
package nearbyshare

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/ossettings"
	"chromiumos/tast/testing"
)

// LaunchAtNearbySettingsPage launches Settings app at nearby settings page.
func LaunchAtNearbySettingsPage(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) error {
	condition := func(ctx context.Context) (bool, error) {
		return ossettings.DescendantNodeExists(ctx, tconn, NearbySettingsUIParams)
	}
	return ossettings.LaunchAtPageURL(ctx, tconn, cr, NearbySettingsSubPageURL, condition)
}

// ChromeConnToNearbySettings returns a Chrome connection to the nearby settings page.
func ChromeConnToNearbySettings(ctx context.Context, cr *chrome.Chrome) (*chrome.Conn, error) {
	nsSettingsFullURL := OsSettingsURLPrefix + NearbySettingsSubPageURL
	nsSettingsConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(nsSettingsFullURL))
	if err != nil {
		return nil, err
	}
	if err := chrome.AddTastLibrary(ctx, nsSettingsConn); err != nil {
		nsSettingsConn.Close()
		return nil, errors.Wrap(err, "failed to introduce tast library")
	}
	return nsSettingsConn, nil
}

// GetParameterStr returns string pass in the test parameter.
func GetParameterStr(s *testing.State, paramName string) string {
	var paramStr string
	if val, ok := s.Var(paramName); ok {
		paramStr = val
	} else {
		s.Fatalf("Need to provide a '%s' argument to 'tast run' command", paramName)
	}
	return paramStr
}

// GetParameterInteger returns integer pass in the test parameter.
func GetParameterInteger(s *testing.State, paramName string) int {
	val := GetParameterStr(s, paramName)
	integer, err := strconv.Atoi(val)
	if err != nil {
		s.Fatalf("Failed to convert '%s' argument (%v) to integer: %v", paramName, val, err)
	}
	return integer
}

// GetParameterSeconds returns time duration in seconds parsed from the test parameter.
func GetParameterSeconds(s *testing.State, paramName string) time.Duration {
	seconds := GetParameterInteger(s, paramName)
	return time.Duration(seconds) * time.Second
}

// SetNearbySettings sets device name, data usage, visibility via nearby mojom API.
func SetNearbySettings(ctx context.Context, s *testing.State, nsSettingsConn *chrome.Conn,
	deviceName string, dataUsage int, visibility int) error {
	var nsSettings chrome.JSObject
	if err := nsSettingsConn.Call(ctx, &nsSettings, `nearbyShare.mojom.NearbyShareSettings.getRemote`); err != nil {
		s.Fatal("Failed to get NearbyShareSetting object: ", err)
	}

	// Device name.
	if err := nsSettings.Call(ctx, nil, `function(name){this.setDeviceName(name);}`, deviceName); err != nil {
		s.Fatal("Failed to set device name in Nearby Sharing settings: ", err)
	}
	var actualDeviceName string
	if err := nsSettings.Call(ctx, &actualDeviceName, `async function() {
			var obj = await this.getDeviceName();
			return obj.deviceName;
		}`); err != nil {
		s.Fatal("Failed to get device name in Nearby Sharing settings: ", err)
	}
	if actualDeviceName != deviceName {
		s.Fatalf("The actual device name '%s' not match given '%s' in Nearby Sharing settings.",
			actualDeviceName, deviceName)
	}
	s.Logf("Successfully set device name to '%s'", deviceName)

	// Data usage.
	if err := nsSettings.Call(ctx, nil, `function(du){this.setDataUsage(du);}`, dataUsage); err != nil {
		s.Fatal("Failed to set data usage in Nearby Sharing settings: ", err)
	}
	var actualDataUsage int
	if err := nsSettings.Call(ctx, &actualDataUsage, `async function() {
			var obj = await this.getDataUsage();
			return obj.dataUsage;
		}`); err != nil {
		s.Fatal("Failed to get data usage in Nearby Sharing settings: ", err)
	}
	if actualDataUsage != dataUsage {
		s.Fatalf("The actual data usage '%d' not match given '%d' in Nearby Sharing settings.",
			actualDataUsage, dataUsage)
	}
	s.Logf("Successfully set data usage to '%d'", dataUsage)

	// Visibility.
	if err := nsSettings.Call(ctx, nil, `function(vis){this.setVisibility(vis);}`, visibility); err != nil {
		s.Fatal("Failed to set visibility in Nearby Sharing settings: ", err)
	}
	var actualVisibility int
	if err := nsSettings.Call(ctx, &actualVisibility, `async function() {
			var obj = await this.getVisibility();
			return obj.visibility;
		}`); err != nil {
		s.Fatal("Failed to get visibility in Nearby Sharing settings: ", err)
	}
	if actualVisibility != visibility {
		s.Fatalf("The actual visibility '%d' not match given '%d' in Nearby Sharing settings.",
			actualVisibility, visibility)
	}
	s.Logf("Successfully set visibility to '%d'", visibility)

	return nil
}

// WaitAndGetEvent waits the specific event within timout.
func WaitAndGetEvent(ctx context.Context, s *testing.State, crConn *chrome.Conn,
	eventName string, pollTmeout time.Duration) (chrome.JSObject, error) {
	var eventData chrome.JSObject
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := crConn.Eval(ctx,
			fmt.Sprintf(`nearbySnippetEventCache.getEvent("%s")`, eventName),
			&eventData); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: pollTmeout}); err != nil {
		s.Fatalf("Timed out after %s waiting for '%s' event: %s",
			pollTmeout, eventName, err)
	}
	return eventData, nil
}

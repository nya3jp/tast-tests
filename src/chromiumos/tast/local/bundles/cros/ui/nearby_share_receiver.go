// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"io/ioutil"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/nearbyshare"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NearbyShareReceiver,
		Desc:         "Makes the chromebook as Nearby receiver to receive the specific file from the specific sender.",
		Contacts:     []string{"ericth@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"nearby_snippet_event_cache.js", "nearby_snippet_receiver.js"},
		Vars:         []string{"receiverName", "senderName", "dataUsage", "visibility", "connectionTimeout", "transferTimeout"},
	})
}

// NearbyShareReceiver tests that we can receive a file from Nearby sender.
func NearbyShareReceiver(ctx context.Context, s *testing.State) {
	// Parse parameters from "tast run" command line.
	receiverName := nearbyshare.GetParameterStr(s, "receiverName")
	senderName := nearbyshare.GetParameterStr(s, "senderName")
	dataUsage := nearbyshare.GetParameterInteger(s, "dataUsage")
	visibility := nearbyshare.GetParameterInteger(s, "visibility")
	connectionTimeout := nearbyshare.GetParameterSeconds(s, "connectionTimeout")
	transferTimeout := nearbyshare.GetParameterSeconds(s, "transferTimeout")
	s.Logf("Get transferTimeout=%s", transferTimeout)

	cr, err := chrome.New(
		ctx,
		chrome.EnableFeatures("IntentHandlingSharing", "NearbySharing", "Sharesheet"),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Launch Nearby Sharing settings subpage.
	if err := nearbyshare.LaunchAtNearbySettingsPage(ctx, tconn, cr); err != nil {
		s.Fatal("Failed to launch Nearby Sharing settings subpage: ", err)
	}

	// Get the Nearby Sharing settings conn.
	nsSettingsConn, err := nearbyshare.ChromeConnToNearbySettings(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get the Nearby Sharing settings Chrome conn: ", err)
	}
	defer nsSettingsConn.Close()

	// Set device name, data usage and device visibility.
	if err := nearbyshare.SetNearbySettings(
		ctx, s, nsSettingsConn, receiverName, dataUsage, visibility); err != nil {
		s.Fatal("Failed to set Nearby settings: ", err)
	}

	// Send JS snippet to the current window.
	for _, jsFileName := range []string{"nearby_snippet_event_cache.js", "nearby_snippet_receiver.js"} {
		dat, err := ioutil.ReadFile(s.DataPath(jsFileName))
		if err != nil {
			s.Fatalf("Failed to read JS snippet file '%s': %s", jsFileName, err)
		}
		if err := nsSettingsConn.Eval(ctx, string(dat), nil); err != nil {
			s.Fatal("Failed to evaluate JS snippet file '%s': %s", jsFileName, err)
		}
	}

	nsSettingsConn.Eval(ctx, `nearbySnippetEventCache.postEvent("onLocalConfirmation", {'receiveSuccess': false})`, nil)

	var eventData string
	if err := nsSettingsConn.Eval(ctx,
		fmt.Sprintf(`nearbySnippetEventCache.getEvent("%s")`, "onLocalConfirmation"),
		&eventData); err != nil {
		s.Log(err)
	}
	s.Log(eventData)

	s.Logf("Receiver '%s' accepts a secure connection from the sender '%s' within %s.",
		receiverName, senderName, connectionTimeout)
	// if eventData, err := nearbyshare.WaitAndGetEvent(ctx, s, nsSettingsConn,
	// 	"onLocalConfirmation", connectionTimeout); err != nil {
	// 	s.Fatal("Failed to receive kAwaitingLocalConfirmation.", err)
	// } else {
	// 	s.Log(eventData)
	// }
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbyshare is used to control Chrome OS Nearby Share functionality.
package nearbyshare

import (
	"time"

	"chromiumos/tast/local/chrome/ui"
)

// ReceiveUIParams are the UI FindParams for the receiving UI's root node.
var ReceiveUIParams ui.FindParams = ui.FindParams{
	Role: ui.RoleTypeRootWebArea,
	Name: "Settings - Nearby Share",
}

// CrosDetectReceiverTimeout is the timeout for a CrOS sender to detect a receiver.
const CrosDetectReceiverTimeout = time.Minute

// CrosDetectSenderTimeout is the timeout for a CrOS receiver to detect a sender.
const CrosDetectSenderTimeout = time.Minute

// SmallFileTimeout is the test timeout for small file transfer tests.
const SmallFileTimeout = 2 * time.Minute

// ChromeLog is the filename of the Chrome log that is saved for each test.
const ChromeLog = "nearby_chrome"

// MessageLog is the filename of the messages log that is saved for each test. It is saved automatically by tast for local tests. For remote tests we need to grab it within the test.
const MessageLog = "nearby_messages"

// NearbyLogDir is the dir that logs will be saved in temporarily on the DUT during remote tests before being pulled back to remote host.
const NearbyLogDir = "/tmp/nearbyshare/"

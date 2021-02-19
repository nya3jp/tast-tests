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
const ChromeLog = "chrome"

// MessageLog is the filename of the messages log that is saved for each test.
const MessageLog = "messages"

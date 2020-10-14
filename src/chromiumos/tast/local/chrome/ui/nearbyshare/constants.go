// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import "chromiumos/tast/local/chrome/ui"

var receiveUIParams ui.FindParams = ui.FindParams{
	Role: ui.RoleTypeRootWebArea,
	Name: "Settings - Nearby Share",
}

var confirmBtnParams ui.FindParams = ui.FindParams{
	Role: ui.RoleTypeButton,
	Name: "Confirm",
}

var incomingShareParams ui.FindParams = ui.FindParams{
	Role: ui.RoleTypeStaticText,
	Name: "Receive from this device?",
}

// SharingContentType is the type of data for sharing.
// These strings will be used to verify that a share was received by checking for the text in the receipt notification.
type SharingContentType string

// List of sharing content types - these appear in the text of the notification after a share is received.
const (
	SharingContentLink SharingContentType = "link"
	SharingContentFile SharingContentType = "file"
)

// ReceivedFollowUpMap maps the SharingContentType to the text of the suggested follow-up action in the receipt notification.
var ReceivedFollowUpMap = map[SharingContentType]string{
	SharingContentLink: "COPY TO CLIPBOARD",
	SharingContentFile: "OPEN FOLDER",
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

const keyType = "uidetection.key_type"
const key = "uidetection.key"
const server = "uidetection.server"
const screenshotSaveDir = "/tmp/sc.png"

// UiDetectionVars contains a list of all variables used by the UI detection library.
var UiDetectionVars = []string{
	keyType,
	key,
	server,
}

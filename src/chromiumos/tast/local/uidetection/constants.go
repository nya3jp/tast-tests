// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

const (
	keyType           = "uidetection.key_type"
	key               = "uidetection.key"
	server            = "uidetection.server"
	screenshotSaveDir = "/tmp"
)

// UIDetectionVars contains a list of all variables used by the UI detection library.
var UIDetectionVars = []string{
	keyType,
	key,
	server,
}

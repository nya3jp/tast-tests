// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uidetection

const (
	// KeyType represents the var name of the key type used for UI detection API.
	KeyType = "uidetection.key_type"
	// Key represents the var name of the key used for UI detection API.
	Key = "uidetection.key"
	// Server represents the var name of the server address used for UI detection API.
	Server            = "uidetection.server"
	screenshotSaveDir = "/tmp"
)

// UIDetectionVars contains a list of all variables used by the UI detection library.
var UIDetectionVars = []string{
	KeyType,
	Key,
	Server,
}

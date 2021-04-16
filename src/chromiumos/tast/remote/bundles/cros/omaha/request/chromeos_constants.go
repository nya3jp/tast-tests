// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package request

// Constants taken from update_engine.
const (
	ProtocolVersion        = "3.0"
	OmahaUpdaterVersion    = "0.1.0.0"
	InstallSourceOnDemand  = "ondemandupdate"
	InstallSourceScheduler = "scheduler"

	OSVersion    = "Indy"
	OSPlatform   = "Chrome OS"
	DefaultAppID = "{87efface-864d-49a5-9bb3-4b050a7c227a}"

	OmahaRequestURL = "https://tools.google.com/service/update2"

	Stable = "stable-channel"
)

const (
	// QAUpdaterID is using a different id to differentiate from real devices.
	QAUpdaterID = "chromeos-qa-updater"

	// ForcedUpdateVersion is used as the version when we want to always get a new version.
	ForcedUpdateVersion = "ForcedUpdate"
)

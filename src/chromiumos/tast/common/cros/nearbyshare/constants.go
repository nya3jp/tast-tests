// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbyshare is used to control Chrome OS Nearby Share functionality.
package nearbyshare

import (
	"time"
)

// DetectShareTargetTimeout is the timeout for a sender to detect an available receiver or vice versa.
const DetectShareTargetTimeout = time.Minute

// SmallFileTransferTimeout is the test timeout for small file (~10kb) transfer tests.
const SmallFileTransferTimeout = 30 * time.Second

// MediumFileOnlineTransferTimeout is the transfer timeout for medium file (~5MB) online transfer tests.
// Online transfers should be at least 10x faster than offline transfers.
// Some extra time is required to account for the delay in upgrading the bandwidth.
const MediumFileOnlineTransferTimeout = 30 * time.Second

// LargeFileOnlineTransferTimeout is the transfer timeout for large file (~30MB) online transfer tests.
const LargeFileOnlineTransferTimeout = time.Minute

// ExtraLargeFileOnlineTransferTimeout is the transfer timeout for extra large file (~100MB) online transfer tests.
const ExtraLargeFileOnlineTransferTimeout = 3 * time.Minute

// AdditionalTestTime is the amount of time to add to the share target detection and file transfer timeouts to make up the global test timeout.
// This is to account for any additional setup and non-sharing interactions performed by the test.
const AdditionalTestTime = 30 * time.Second

// DetectionTimeout is the standard timeout for activities performed in a test excluding the actual file transfer.
// 2*DetectShareTargetTimeout accounts for the amount of time given for the sender to find+select the receiver,
// and then for the receiver to detect the incoming share from the sender.
const DetectionTimeout = 2*DetectShareTargetTimeout + AdditionalTestTime

// ChromeLog is the filename of the Chrome log that is saved for each test.
const ChromeLog = "nearby_chrome"

// MessageLog is the filename of the messages log that is saved for each test. It is saved automatically by tast for local tests. For remote tests we need to grab it within the test.
const MessageLog = "nearby_messages"

// BtsnoopLog is the filename of the Chrome OS btsnoop log that is saved for each test.
const BtsnoopLog = "nearby_btsnoop_cros.log"

// NearbyLogDir is the dir that logs will be saved in temporarily on the DUT during remote tests before being pulled back to remote host.
const NearbyLogDir = "/tmp/nearbyshare/"

// KeepStateVar is the runtime variable name used to specify the chrome.KeepState parameter to preserve the DUT's user accounts.
const KeepStateVar = "keepState"

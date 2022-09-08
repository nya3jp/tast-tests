// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gaiaenrollment

// TestParams is a helper struct to keep parameters for gaia enrollment tests.
type TestParams struct {
	DMServer             string // device management server url
	PoolID               string // poolID for the used test account
	SerialNumber         string // serial number for the ZTE device to be pre-provisioned
	HardwareModel        string // hardware model for the ZTE device to be pre-provisioned
	DeviceProvisionToken string // device provision token for the ZTE device to be pre-provisioned
	CustomerID           string // customer id of the domain where the ZTE device will be enrolled
	BatchKey             string // batch key required for the pre provision curl command to succeed
}

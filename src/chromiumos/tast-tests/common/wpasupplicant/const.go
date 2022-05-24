// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wpasupplicant provides common constants of wpa_supplicant that can
// be used in both local and remote libraries/tests.
package wpasupplicant

// The following are some of the expected values of the property DisconnectReason.
const (
	// DisconnReasonPreviousAuthenticationInvalid previous authentication no longer valid.
	DisconnReasonPreviousAuthenticationInvalid = 2
	// DisconnReasonDeauthSTALeaving deauthenticated because sending STA is leaving (or has left) IBSS or ESS.
	DisconnReasonDeauthSTALeaving = 3
	// DisconnReasonLGDeauthSTALeaving (locally generated).
	DisconnReasonLGDeauthSTALeaving = -3
	// DisconnReasonLGDisassociatedInactivity (locally generated) disassociated due to inactivity.
	DisconnReasonLGDisassociatedInactivity = -4
	// DisconnReasonUnknown.
	DisconnReasonUnknown = 0
)

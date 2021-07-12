// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

// Signal names of power manager D-Bus API.
const (
	SignalPowerSupplyPoll     = "PowerSupplyPoll"
	SignalSuspendImminent     = "SuspendImminent"
	SignalDarkSuspendImminent = "DarkSuspendImminent"
	SignalSuspendDone         = "SuspendDone"
)

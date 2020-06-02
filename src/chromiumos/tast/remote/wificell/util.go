// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"

	"chromiumos/tast/caller"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

// collectFirstErr collects the first error into firstErr and logs the others.
// This can be useful when you have several steps in a function but cannot early
// return on error. e.g. cleanup functions.
func collectFirstErr(ctx context.Context, firstErr *error, err error) {
	if err == nil {
		return
	}
	testing.ContextLogf(ctx, "Error in %s: %s", caller.Get(2), err)
	if *firstErr == nil {
		*firstErr = err
	}
}

// CommonAPOptions generates a set of options with common settings on protocol
// and channel appended with given extra options.
// This function is useful for the tests which don't quite care about the
// protocol used but need to set some other options like SSID or security.
func CommonAPOptions(extraOps ...hostapd.Option) []hostapd.Option {
	// Common case: 80211n, 5GHz channel, 40 MHz width.
	commonOps := []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211nMixed),
		hostapd.Channel(48),
		hostapd.HTCaps(hostapd.HTCapHT40),
	}
	// Append extra options.
	return append(commonOps, extraOps...)
}

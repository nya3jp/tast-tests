// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/local/stork"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: StorkProfile,
		Desc: "Verifies that the Stork API can be invoked by a device",
		Contacts: []string{
			"khorimoto@google.com",
			"pholla@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:    []string{"group:cellular", "cellular_unstable"},
		Timeout: 5 * time.Minute,
	})
}

func StorkProfile(ctx context.Context, s *testing.State) {
	activationCode, cleanupFunc, err := stork.FetchStorkProfile(ctx)
	if err != nil {
		s.Error("Failed to fetch Stork profile: ", err)
	}

	defer cleanupFunc(ctx)
	s.Log("Fetched Stork profile with activation code: ", activationCode)
}

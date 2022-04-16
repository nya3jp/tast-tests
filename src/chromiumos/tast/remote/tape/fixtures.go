// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tape

import (
	"chromiumos/tast/common/tape"
	"chromiumos/tast/testing"
)

func init() {
	// Initialize all remote tape fixtures.
	remoteFixtures := tape.RemoteFixtures()
	for _, curFixture := range remoteFixtures {
		testing.AddFixture(curFixture)
	}
}

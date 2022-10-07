// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LocalRemoteFixture,
		Desc:         "Tests local tests can depend on remote fixtures",
		Contacts:     []string{"tast-owners@google.com", "oka@chromium.org"},
		BugComponent: "b:1034625",
		Fixture:      "metaRemote",
	})
}

func LocalRemoteFixture(ctx context.Context, s *testing.State) {
	s.Log("Hello test")
}

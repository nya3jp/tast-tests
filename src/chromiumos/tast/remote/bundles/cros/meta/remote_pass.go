// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     RemotePass,
		Desc:     "Always passes",
		Contacts: []string{"tast-owners@google.com"},
	})
}

func RemotePass(ctx context.Context, s *testing.State) {
}

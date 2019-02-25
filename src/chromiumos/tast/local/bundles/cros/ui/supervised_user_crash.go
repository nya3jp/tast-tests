// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/bundles/cros/ui/supervised"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SupervisedUserCrash,
		Desc:         "Signs in, indicates that supervised user is being created, then crashes",
		Contacts:     []string{"hidehiko@chromium.org"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

func SupervisedUserCrash(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to log in using Chrome: ", err)
	}
	defer cr.Close(ctx)
	supervised.RunTest(ctx, s)
}

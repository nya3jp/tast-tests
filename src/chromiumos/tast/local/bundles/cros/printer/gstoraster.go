// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"

	"chromiumos/tast/local/bundles/cros/printer/gstoraster"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Gstoraster,
		Desc:         "Tests that the gstoraster CUPS filter produces expected output",
		Contacts:     []string{"valleau@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", "cups"},
		Data:         []string{"gstoraster_input.pdf", "gstoraster_golden.pwg"},
		Pre:          chrome.LoggedIn(),
	})
}

func Gstoraster(ctx context.Context, s *testing.State) {
	gstoraster.RunGstorasterTest(ctx, s, s.DataPath("gstoraster_input.pdf"), s.DataPath("gstoraster_golden.pwg"))
}

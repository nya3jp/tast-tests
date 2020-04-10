// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"fmt"
	"net/http"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/testing"
)

// InformStatusServlet informs status servlet about the progress of detached cases
func InformStatusServlet(ctx context.Context, s *testing.State, statusSvr string, action string, dutID string) {
	statusURL := "%v?action=%v&dut=%v&testCase=wifi.MTBF058WifiDownload"
	statusServletURL := fmt.Sprintf(statusURL, statusSvr, action, dutID)
	s.Log("Access statusURL: ", statusServletURL)

	if _, err := http.Get(statusServletURL); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.NotifyDetachSvr, err, statusServletURL))
	}
}

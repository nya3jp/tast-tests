// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/httputil"
	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/testing"
)

// InformStatusServlet informs status servlet about the progress of detached cases.
func InformStatusServlet(ctx context.Context, s *testing.State, statusSvr string, action string, dutID string, caseName string) {
	statusURL := "%v?action=%v&dut=%v&testCase=%v"
	statusServletURL := fmt.Sprintf(statusURL, statusSvr, action, dutID, caseName)
	s.Log("Will access statusURL: ", statusServletURL)
	retryCnt := 3
	retry := 0
	var err error
	var response string

	for retry < retryCnt {
		response, err := httputil.HTTPGetStr(statusServletURL, 30*time.Second)

		if err == nil {
			s.Log("detachStatus servlet has been notified. response: ", response)
			return
		}

		s.Log("Failed notify detachStatus servlet ethernet: ", err)
		s.Log("retry: ", retry)
		retry++
	}

	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.NotifyDetachSvr, err, statusServletURL))
	}

	s.Log("Response from status servlet: '", response, "'")
}

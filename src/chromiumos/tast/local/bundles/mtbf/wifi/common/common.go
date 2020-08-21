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
func InformStatusServlet(ctx context.Context, s *testing.State, statusSvr, action, dutID, caseName string) {
	statusURL := "%v?action=%v&dut=%v&testCase=%v"
	statusServletURL := fmt.Sprintf(statusURL, statusSvr, action, dutID, caseName)
	s.Log("Will access statusURL: ", statusServletURL)
	retryCnt := 3
	retry := 0
	var err error
	var response string
	var statusCode int

	for retry < retryCnt {
		response, statusCode, err = httputil.HTTPGetStr(statusServletURL, 30*time.Second)

		if err == nil {
			s.Logf("detachStatus servlet has been notified. statusCode:%v, response: %v", statusCode, response)
			return
		}

		s.Logf("Failed notify detachStatus servlet. retry: %v, statusCode: %v, err: %v", retry, statusCode, err)
		retry++
	}

	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.NotifyDetachSvr, err, statusServletURL))
	}

	s.Logf("Response from status servlet: statusCode=%v response=%q", statusCode, response)
}

// Sleep sleep a period of time and do error handling
func Sleep(ctx context.Context, s *testing.State, duration time.Duration) {
	if err := testing.Sleep(ctx, duration); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.WIFISleep, err))
	}
}

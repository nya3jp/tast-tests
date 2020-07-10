// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"strings"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SysInfoPII,
		Desc:         "Verify that known-sensitive data doesn't show up in feedback reports",
		Contacts:     []string{"mutexlox@google.com", "cros-telemetry@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// systemInformation corresponds to the "SystemInformation" defined in autotest_private.idl.
type systemInformation struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func SysInfoPII(ctx context.Context, s *testing.State) {
	const (
		sensitiveURL   = "https://www.google.com/search?q=qwertyuiopasdfghjkl+sensitive"
		sensitiveTitle = "qwertyuiopasdfghjkl sensitive"
	)

	cr := s.PreValue().(*chrome.Chrome)
	conn, err := cr.NewConn(ctx, "")
	if err != nil {
		s.Fatal("Failed to create renderer: ", err)
	}
	defer conn.Close()

	if err := conn.Navigate(ctx, sensitiveURL); err != nil {
		s.Fatal("Failed to open tab: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Could not create test API conn: ", err)
	}
	var ret []*systemInformation
	if err := tconn.Eval(ctx, "tast.promisify(chrome.autotestPrivate.getSystemInformation)()", &ret); err != nil {
		s.Fatal("Could not call getSystemInformation: ", err)
	}

	for _, info := range ret {
		if info.Key == "mem_usage_with_title" {
			if !strings.Contains(info.Value, sensitiveTitle) {
				s.Errorf("Log %q unexpectedly did not contain tab name", info.Key)
			}
		} else {
			if strings.Contains(info.Value, sensitiveTitle) {
				s.Errorf("Log %q unexpectedly contained tab name", info.Key)
			}

			if strings.Contains(info.Value, sensitiveURL) {
				s.Errorf("Log %q unexpectedly contained URL", info.Key)
			}
		}
	}
}

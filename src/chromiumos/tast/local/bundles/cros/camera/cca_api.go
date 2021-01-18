// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAAPI,
		Desc:         "Verifies that the private JavaScript APIs CCA relies on work as expected",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Pre:          chrome.LoggedIn(),
	})
}

// CCAAPI verifies whether the private JavaScript APIs CCA (Chrome camera app) relies on work as
// expected. The APIs under testing are not owned by CCA team. This test prevents changes to those
// APIs' implementations from silently breaking CCA.
func CCAAPI(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	conn, err := cr.NewConn(ctx, "chrome://camera-app/views/test.html")
	if err != nil {
		s.Fatal("Failed to connect to CCA test page: ", err)
	}

	result := true
	if err := conn.Eval(ctx, "window.FileSystemHandle !== undefined", &result); err != nil {
		s.Fatal("Failed to evaluate codes on the test page: ", err)
	} else if !result {
		s.Error("window.FileSystemHandle is not available on the test page")
	}

	if err := conn.Eval(ctx, "window.launchQueue !== undefined", &result); err != nil {
		s.Fatal("Failed to evaluate codes on the test page: ", err)
	} else if !result {
		s.Error("window.launchQueue is not available on the test page")
	}

	if err := conn.Eval(ctx, `
	  (async function() {
	    await import('/strings.m.js');
	    return window.loadTimeData !== undefined;
	  })();
	`, &result); err != nil {
		s.Fatal("Failed to evaluate codes on the test page: ", err)
	} else if !result {
		s.Error("window.loadTimeData is not available on the test page")
	}
}

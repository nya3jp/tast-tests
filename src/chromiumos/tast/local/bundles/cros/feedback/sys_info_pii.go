// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"

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
	key   string `json:"key"`
	value string `json:"value"`
}

func SysInfoPII(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Could not create test API conn: ", err)
	}
	var ret []*systemInformation
	if err := tconn.Eval(ctx, "tast.promisify(chrome.autotestPrivate.getSystemInformation)()", &ret); err != nil {
		s.Fatal("Could not call getSystemInformation: ", err)
	}
	// TODO(mutexlox): There are 201 items in this list, which is right, but
	// each item's key and value are empty. why?
	s.Logf("%d items", len(ret))
	for _, info := range ret {
		s.Logf("%s: %q; %d: %d", info.key, info.value, len(info.key), len(info.value))
	}
}

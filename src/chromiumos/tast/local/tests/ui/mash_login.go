// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"chromiumos/tast/common/testing"
	"chromiumos/tast/local/chrome"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MashLogin,
		Desc: "Checks that chrome --mash starts",
		Attr: []string{"bvt", "chrome"},
	})
}

// MashLogin checks that chrome --mash starts.
func MashLogin(s *testing.State) {
	cr, err := chrome.New(s.Context(), chrome.MashEnabled())
	if err != nil {
		s.Fatal("Chrome probably crashed on startup: ", err)
	}
	defer cr.Close(s.Context())
}

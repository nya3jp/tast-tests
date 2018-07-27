// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeLoginForever,
		Desc:         "Checks that Chrome login succeeds repeatedly",
		Attr:         []string{"disabled"},
		SoftwareDeps: []string{"chrome_login"},
		Timeout:      365 * 24 * time.Hour,
	})
}

func ChromeLoginForever(s *testing.State) {
	iter := func() {
		ctx, cancel := context.WithTimeout(s.Context(), 180*time.Second)
		defer cancel()

		cr, err := chrome.New(ctx)
		if err != nil {
			s.Fatal("Failed to connect to Chrome: ", err)
		}
		defer cr.Close(ctx)

		// Skip further sanity checks to speed up iterations.
	}

	for i := 1; ; i++ {
		s.Log("======= Iteration ", i)
		iter()
	}
}

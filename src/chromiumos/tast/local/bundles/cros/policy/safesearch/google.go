// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package safesearch

import (
	"context"

	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/testing"
)

// IsGoogleSafeSearchEnabled checks whether safe search is automatically enabled
// for Google search.
func IsGoogleSafeSearchEnabled(ctx context.Context, s *testing.State, br *browser.Browser) bool {
	conn, err := br.NewConn(ctx, "https://www.google.com/search?q=kittens")
	if err != nil {
		s.Fatal("Failed to connect to chrome: ", err)
	}
	defer conn.Close()

	var isSafe bool
	if err := conn.Eval(ctx, `new URL(document.URL).searchParams.get("safe") == "active"`, &isSafe); err != nil {
		s.Fatal("Could not read safe search param from URL: ", err)
	}
	return isSafe
}

// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package safesearch

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/browser"
)

// TestGoogleSafeSearch checks whether safe search is automatically enabled for
// Google search.
func TestGoogleSafeSearch(ctx context.Context, br *browser.Browser, safeSearchExpected bool) error {
	conn, err := br.NewConn(ctx, "https://www.google.com/search?q=kittens")
	if err != nil {
		return errors.Wrap(err, "failed to connect to Chrome")
	}
	defer conn.Close()

	var isSafe bool
	if err := conn.Eval(ctx, `new URL(document.URL).searchParams.get("safe") == "active"`, &isSafe); err != nil {
		return errors.Wrap(err, "could not read safe search param from URL")
	}

	if isSafe != safeSearchExpected {
		return errors.Errorf("unexpected safe search behavior; got %t, want %t", isSafe, safeSearchExpected)
	}

	return nil
}

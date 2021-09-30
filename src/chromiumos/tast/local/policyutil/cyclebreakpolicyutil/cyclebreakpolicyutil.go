// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cyclebreakpolicyutil

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/lacros"
)

// BrowserSetup will do the setup based on the chrome.
// It should take a fixture which implements the chrome.HasChrome interface and chrome type and
// returns a browser and a cleanup function that accepts cleanup context that should be deferred.
// Example of usage:
//
//	br, cleanup, err := cyclebreakpolicyutil.BrowserSetup(ctx, s.FixtValue(), s.Param().(lacros.ChromeType))
//	if err != nil {
//		s.Fatal("Failed to open the browser: ", err)
//	}
//	defer cleanup(cleanupCtx)
func BrowserSetup(ctx context.Context, fixt interface{}, crt lacros.ChromeType) (ash.ConnSource, func(context.Context), error) {
	_, ok := fixt.(chrome.HasChrome)
	if !ok {
		return nil, nil, errors.Errorf("unrecognized fixture type: %v", fixt)
	}

	_, l, br, err := lacros.Setup(ctx, fixt, crt)
	return br, func(c context.Context) {
		lacros.CloseLacrosChrome(c, l)
	}, err
}

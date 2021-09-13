// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacrospolicyutil

import (
	"context"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/lacros"
)

// PolicyFixtData is an interface to get Chrome and FakeDMS.
type PolicyFixtData interface {
	GetChrome() *chrome.Chrome
	GetFakeDMS() *fakedms.FakeDMS
}

// BrowserSetup will do the setup based on the chrome type and returns browser and a cleanup function that should be deferred.
func BrowserSetup(ctx context.Context, f interface{}, crt lacros.ChromeType) (ash.ConnSource, func(context.Context), error) {
	_, l, br, err := lacros.Setup(ctx, f, crt)
	return br, func(c context.Context) {
		lacros.CloseLacrosChrome(c, l)
	}, err
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/wpr"
	"chromiumos/tast/testing"
)

// NewLacrosWPRFixture creates a new fixture that can launch Lacros chrome with the given setup mode,
// Chrome options, and WPR archive. This should be the child of a WPR fixture.
func NewLacrosWPRFixture(mode SetupMode, fOpt chrome.OptionsCallback) testing.FixtureImpl {
	return NewComposedFixture(mode, func(v FixtValue, pv interface{}) interface{} {
		return v
	}, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
		opts, err := s.ParentValue().(wpr.FixtValue).FOpt()(ctx, s)
		if err != nil {
			return nil, err
		}

		optsExtra, err := fOpt(ctx, s)
		if err != nil {
			return nil, err
		}

		opts = append(opts, optsExtra...)
		return opts, nil
	})
}

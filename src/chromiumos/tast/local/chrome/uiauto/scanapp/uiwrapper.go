// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scanapp

import (
	"time"

	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
)

// This file is used for methods on Scan App that just wrap methods on uiauto.Context.
// Not all methods are necessarily added. Please add them if you need them.

// WithTimeout returns a new ScanApp with the specified timeout.
// This does not launch the Scan App again.
func (s *ScanApp) WithTimeout(timeout time.Duration) *ScanApp {
	return &ScanApp{
		ui:    s.ui.WithTimeout(timeout),
		tconn: s.tconn,
	}
}

// WaitUntilExists calls ui.WaitUntilExists scoping the finder to the Scan App.
func (s *ScanApp) WaitUntilExists(finder *nodewith.Finder) uiauto.Action {
	return s.ui.WaitUntilExists(finder.FinalAncestor(WindowFinder))
}

// WaitUntilGone calls ui.WaitUntilGone scoping the finder to the Scan App.
func (s *ScanApp) WaitUntilGone(finder *nodewith.Finder) uiauto.Action {
	return s.ui.WaitUntilGone(finder.FinalAncestor(WindowFinder))
}

// LeftClick calls ui.LeftClick scoping the finder to the Scan App.
func (s *ScanApp) LeftClick(finder *nodewith.Finder) uiauto.Action {
	return s.ui.LeftClick(finder.FinalAncestor(WindowFinder))
}

// MakeVisible calls ui.MakeVisible scoping the finder to the Scan App.
func (s *ScanApp) MakeVisible(finder *nodewith.Finder) uiauto.Action {
	return s.ui.MakeVisible(finder.FinalAncestor(WindowFinder))
}

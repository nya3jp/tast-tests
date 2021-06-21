// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ossettings

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

// This file is used for methods on Settings app that just wrap methods on uiauto.Context.
// Not all methods are necessarily added. Please add them if you need them.

// WithTimeout returns a new OSSettings with the specified timeout.
// This does not launch the Settings app again.
func (s *OSSettings) WithTimeout(timeout time.Duration) *OSSettings {
	return &OSSettings{
		ui:    s.ui.WithTimeout(timeout),
		tconn: s.tconn,
	}
}

// WithInterval returns a new OSSettings with the specified polling interval.
// This does not launch the Settings app again.
func (s *OSSettings) WithInterval(interval time.Duration) *OSSettings {
	return &OSSettings{
		ui:    s.ui.WithInterval(interval),
		tconn: s.tconn,
	}
}

// WithPollOpts returns a new OSSettings with the specified polling options.
// This does not launch the Settings app again.
func (s *OSSettings) WithPollOpts(pollOpts testing.PollOptions) *OSSettings {
	return &OSSettings{
		ui:    s.ui.WithPollOpts(pollOpts),
		tconn: s.tconn,
	}
}

// Info calls ui.Info scoping the finder to the Settings app.
func (s *OSSettings) Info(ctx context.Context, finder *nodewith.Finder) (*uiauto.NodeInfo, error) {
	return s.ui.Info(ctx, finder.FinalAncestor(WindowFinder))
}

// Exists calls ui.Exists scoping the finder to the Settings app.
func (s *OSSettings) Exists(finder *nodewith.Finder) uiauto.Action {
	return s.ui.Exists(finder.FinalAncestor(WindowFinder))
}

// WaitUntilExists calls ui.WaitUntilExists scoping the finder to the Settings app.
func (s *OSSettings) WaitUntilExists(finder *nodewith.Finder) uiauto.Action {
	return s.ui.WaitUntilExists(finder.FinalAncestor(WindowFinder))
}

// Gone calls ui.Gone scoping the finder to the Settings app.
func (s *OSSettings) Gone(finder *nodewith.Finder) uiauto.Action {
	return s.ui.Gone(finder.FinalAncestor(WindowFinder))
}

// WaitUntilGone calls ui.WaitUntilGone scoping the finder to the Settings app.
func (s *OSSettings) WaitUntilGone(finder *nodewith.Finder) uiauto.Action {
	return s.ui.WaitUntilGone(finder.FinalAncestor(WindowFinder))
}

// LeftClick calls ui.LeftClick scoping the finder to the Settings app.
func (s *OSSettings) LeftClick(finder *nodewith.Finder) uiauto.Action {
	return s.ui.LeftClick(finder.FinalAncestor(WindowFinder))
}

// RightClick calls ui.RightClick scoping the finder to the Settings app.
func (s *OSSettings) RightClick(finder *nodewith.Finder) uiauto.Action {
	return s.ui.RightClick(finder.FinalAncestor(WindowFinder))
}

// DoubleClick calls ui.DoubleClick scoping the finder to the Settings app.
func (s *OSSettings) DoubleClick(finder *nodewith.Finder) uiauto.Action {
	return s.ui.DoubleClick(finder.FinalAncestor(WindowFinder))
}

// LeftClickUntil calls ui.LeftClickUntil scoping the finder to the Settings app.
func (s *OSSettings) LeftClickUntil(finder *nodewith.Finder, condition func(context.Context) error) uiauto.Action {
	return s.ui.LeftClickUntil(finder.FinalAncestor(WindowFinder), condition)
}

// FocusAndWait calls ui.FocusAndWait scoping the finder to the Settings app.
func (s *OSSettings) FocusAndWait(finder *nodewith.Finder) uiauto.Action {
	return s.ui.FocusAndWait(finder.FinalAncestor(WindowFinder))
}

// MakeVisible calls ui.MakeVisible scoping the finder to the Settings app.
func (s *OSSettings) MakeVisible(finder *nodewith.Finder) uiauto.Action {
	return s.ui.MakeVisible(finder.FinalAncestor(WindowFinder))
}

// NodesInfo calls ui.NodesInfo scoping the finder to the Settings app.
func (s *OSSettings) NodesInfo(ctx context.Context, finder *nodewith.Finder) ([]uiauto.NodeInfo, error) {
	return s.ui.NodesInfo(ctx, finder.FinalAncestor(WindowFinder))
}

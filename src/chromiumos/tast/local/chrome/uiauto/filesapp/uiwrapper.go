// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filesapp

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

// This file is used for methods on Files app that just wrap methods on uiauto.Context.
// Not all methods are necessarily added. Please add them if you need them.

// WithTimeout returns a new FilesApp with the specified timeout.
// This does not launch the Files App again.
func (f *FilesApp) WithTimeout(timeout time.Duration) *FilesApp {
	return &FilesApp{
		ui:    f.ui.WithTimeout(timeout),
		tconn: f.tconn,
		appID: f.appID,
	}
}

// WithInterval returns a new FilesApp with the specified polling interval.
// This does not launch the Files App again.
func (f *FilesApp) WithInterval(interval time.Duration) *FilesApp {
	return &FilesApp{
		ui:    f.ui.WithInterval(interval),
		tconn: f.tconn,
		appID: f.appID,
	}
}

// WithPollOpts returns a new FilesApp with the specified polling options.
// This does not launch the Files App again.
func (f *FilesApp) WithPollOpts(pollOpts testing.PollOptions) *FilesApp {
	return &FilesApp{
		ui:    f.ui.WithPollOpts(pollOpts),
		tconn: f.tconn,
		appID: f.appID,
	}
}

// Info calls ui.Info scoping the finder to the Files App.
func (f *FilesApp) Info(ctx context.Context, finder *nodewith.Finder) (*uiauto.NodeInfo, error) {
	return f.ui.Info(ctx, finder.FinalAncestor(WindowFinder(f.appID)))
}

// NodesInfo calls ui.NodesInfo scoping the finder to the Files App.
func (f *FilesApp) NodesInfo(ctx context.Context, finder *nodewith.Finder) ([]uiauto.NodeInfo, error) {
	return f.ui.NodesInfo(ctx, finder.FinalAncestor(WindowFinder(f.appID)))
}

// Exists calls ui.Exists scoping the finder to the Files App.
func (f *FilesApp) Exists(finder *nodewith.Finder) uiauto.Action {
	return f.ui.Exists(finder.FinalAncestor(WindowFinder(f.appID)))
}

// IsNodeFound calls ui.IsNodeFound scoping the finder to the Files App.
func (f *FilesApp) IsNodeFound(ctx context.Context, finder *nodewith.Finder) (bool, error) {
	return f.ui.IsNodeFound(ctx, finder.FinalAncestor(WindowFinder(f.appID)))
}

// WaitUntilExists calls ui.WaitUntilExists scoping the finder to the Files App.
func (f *FilesApp) WaitUntilExists(finder *nodewith.Finder) uiauto.Action {
	return f.ui.WaitUntilExists(finder.FinalAncestor(WindowFinder(f.appID)))
}

// Gone calls ui.Gone scoping the finder to the Files App.
func (f *FilesApp) Gone(finder *nodewith.Finder) uiauto.Action {
	return f.ui.Gone(finder.FinalAncestor(WindowFinder(f.appID)))
}

// WaitUntilGone calls ui.WaitUntilGone scoping the finder to the Files App.
func (f *FilesApp) WaitUntilGone(finder *nodewith.Finder) uiauto.Action {
	return f.ui.WaitUntilGone(finder.FinalAncestor(WindowFinder(f.appID)))
}

// EnsureGoneFor calls ui.EnsureGoneFor scoping the finder to the Files App.
func (f *FilesApp) EnsureGoneFor(finder *nodewith.Finder, duration time.Duration) uiauto.Action {
	return f.ui.EnsureGoneFor(finder.FinalAncestor(WindowFinder(f.appID)), duration)
}

// LeftClick calls ui.LeftClick scoping the finder to the Files App.
func (f *FilesApp) LeftClick(finder *nodewith.Finder) uiauto.Action {
	return f.ui.LeftClick(finder.FinalAncestor(WindowFinder(f.appID)))
}

// RightClick calls ui.RightClick scoping the finder to the Files App.
func (f *FilesApp) RightClick(finder *nodewith.Finder) uiauto.Action {
	return f.ui.RightClick(finder.FinalAncestor(WindowFinder(f.appID)))
}

// DoubleClick calls ui.DoubleClick scoping the finder to the Files App.
func (f *FilesApp) DoubleClick(finder *nodewith.Finder) uiauto.Action {
	return f.ui.DoubleClick(finder.FinalAncestor(WindowFinder(f.appID)))
}

// LeftClickUntil calls ui.LeftClickUntil scoping the finder to the Files App.
func (f *FilesApp) LeftClickUntil(finder *nodewith.Finder, condition func(context.Context) error) uiauto.Action {
	return f.ui.LeftClickUntil(finder.FinalAncestor(WindowFinder(f.appID)), condition)
}

// FocusAndWait calls ui.FocusAndWait scoping the finder to the Files App.
func (f *FilesApp) FocusAndWait(finder *nodewith.Finder) uiauto.Action {
	return f.ui.FocusAndWait(finder.FinalAncestor(WindowFinder(f.appID)))
}

// EnsureFocused calls ui.FocusAndWait if the target node is not focused.
func (f *FilesApp) EnsureFocused(finder *nodewith.Finder) uiauto.Action {
	return f.ui.EnsureFocused(finder.FinalAncestor(WindowFinder(f.appID)))
}

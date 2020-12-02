// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"path"
	"strings"
	"time"

	"chromiumos/tast/caller"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/wpr"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// wprTimeout is the time to wait for WPR sockets.
const wprTimeout = 10 * time.Second

// getCallerPackage returns the package name of the caller of ReplayMode and
// RecordMode.
func getCallerPackage() string {
	const replayModeRecordModeCaller = 3
	c := caller.Get(replayModeRecordModeCaller)
	pkg := strings.SplitN(c, ".", 2)[0]
	return path.Base(pkg)
}

// KeepStateReplayMode returns a precondition that WPR is started in replay mode using
// the given archive as data file of the package and Chrome is logged in and
// redirects its traffic through WPR.
//
// The precondition is keyed by pkg and archive. Tests of the same package
// and the same archive would use the same precondition instance and save
// the time to start WPR and Chrome. Pkg is determined by caller.Get(). Test
// must supply the name of the archive.
//
// Example usage:
//
//	func init() {
//		testing.AddTest(&testing.Test{
//			Func: DoSomething
//			...
//			Data: []string{
//				...,
//				"example_wpr_archive.wprgo"
//			},
//			Pre: wpr.ReplayMode("example_wpr_archive.wprgo"),
//		})
//	}
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		// cr is a logged-in Chrome with net traffic redirected to WPR.
//		cr := s.PreValue().(*chrome.Chrome)
//		...
//	}
func KeepStateReplayMode(archive string) testing.Precondition {
	return getOrCreateKeepStatePrecondition(getCallerPackage(), archive, wpr.Replay)
}

// KeepStateRecordMode returns a precondition similar to the ReplayMode above except
// WPR runs in record mode and all sites accessed by Chrome are recorded
// in the given archive path on the device.
//
// Example usage:
//
//	func init() {
//		testing.AddTest(&testing.Test{
//			Func: DoSomething
//			...
//			Pre: wpr.RecordMode("/tmp/example_wpr_archive.wprgo"),
//		})
//	}
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		// cr is a logged-in Chrome with net traffic redirected through WPR
//    // and recorded.
//		cr := s.PreValue().(*chrome.Chrome)
//		...
//	}
func KeepStateRecordMode(archive string) testing.Precondition {
	return getOrCreateKeepStatePrecondition(getCallerPackage(), archive, wpr.Record)
}

// wprKeepStateImpl implements testing.Precondition.
type wprKeepStateImpl struct {
	// Data for testing.Precondition.
	name    string
	timeout time.Duration

	// mode in which WPR runs.
	mode wpr.Mode
	// archive represents the path that WPR replays from or records to.
	archive string

	// WPR instance.
	wpr *wpr.WPR

	// Chrome instance that runs with WPR Chrome options.
	cr *chrome.Chrome
}

func (p *wprKeepStateImpl) String() string         { return p.name }
func (p *wprKeepStateImpl) Timeout() time.Duration { return p.timeout }

func (p *wprKeepStateImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	if p.cr != nil && p.wpr != nil {
		err := func() error {
			ctx, cancel := context.WithTimeout(ctx, resetTimeout)
			defer cancel()
			ctx, st := timing.Start(ctx, "reset_chrome_"+p.name)
			defer st.End()
			if err := p.cr.Responded(ctx); err != nil {
				return errors.Wrap(err, "existing Chrome connection is unusable")
			}
			if err := p.cr.ResetState(ctx); err != nil {
				return errors.Wrap(err, "failed resetting existing Chrome session")
			}
			return nil
		}()
		if err == nil {
			s.Log("Reusing existing Chrome/WPR session")
			return p.cr
		}

		s.Log("Failed to reuse existing Chrome session: ", err)
		p.Close(ctx, s)
	}

	func() {
		var archive string
		switch p.mode {
		case wpr.Replay:
			archive = s.DataPath(p.archive)
		case wpr.Record:
			archive = p.archive
		default:
			s.Fatal("Unknown WPR mode: ", p.mode)
		}

		// Use s.PreCtx() to create WPR instance because the WPR process
		// needs live beyond the |ctx| associated with Prepare stage.
		var err error
		if p.wpr, err = wpr.New(s.PreCtx(), p.mode, archive); err != nil {
			s.Fatal("Failed to start WPR: ", err)
		}
	}()

	testing.ContextLogf(ctx, "Starting Chrome with WPR at ports %d and %d",
		p.wpr.HTTPPort, p.wpr.HTTPSPort)
	opts := append(p.wpr.ChromeOptions, chrome.KeepState())
	var err error
	p.cr, err = chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	chrome.Lock()

	return p.cr
}

func (p *wprKeepStateImpl) Close(ctx context.Context, s *testing.PreState) {
	if p.cr != nil {
		chrome.Unlock()
		if err := p.cr.Close(ctx); err != nil {
			s.Fatal("Failed to close Chrome connection: ", err)
		}
		p.cr = nil
	}

	if p.wpr != nil {
		if err := p.wpr.Close(ctx); err != nil {
			s.Fatal("Failed to stop wpr: ", err)
		}
		p.wpr = nil
	}
}

// preKeepStateMapKey holds variations that a wprKeepStateImpl instance could have.
type preKeepStateMapKey struct {
	mode    wpr.Mode
	pkg     string
	archive string
}

func (k *preKeepStateMapKey) String() string {
	return k.mode.String() + "_" + k.pkg + "_" + k.archive
}

// preKeepStateMap is a map to track wprKeepStateImpl instances keyed by caller package name and
// WPR params.
var preKeepStateMap = make(map[preKeepStateMapKey]*wprKeepStateImpl)

// getOrCreateKeepStatePrecondition gets existing instance of precondition that matches
// the given params and creates one if none of the existing instances matches.
func getOrCreateKeepStatePrecondition(pkg, archive string, mode wpr.Mode) *wprKeepStateImpl {
	k := preKeepStateMapKey{
		mode:    mode,
		pkg:     pkg,
		archive: archive,
	}
	if pre, ok := preKeepStateMap[k]; ok {
		return pre
	}

	pre := &wprKeepStateImpl{
		name:    "wpr_keepstate_" + k.String(),
		timeout: resetTimeout + wprTimeout + chrome.LoginTimeout,
		mode:    mode,
		archive: archive,
	}
	preKeepStateMap[k] = pre
	return pre
}

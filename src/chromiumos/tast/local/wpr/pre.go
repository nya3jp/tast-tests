// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpr

import (
	"context"
	"path"
	"strings"
	"time"

	"chromiumos/tast/caller"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// wprTimeout is the time to wait for WPR sockets.
const wprTimeout = 10 * time.Second

// resetTimeout is the timeout duration to trying reset of the current precondition.
const resetTimeout = 15 * time.Second

// getCallerPackage returns the package name of the caller of ReplayMode and
// RecordMode.
func getCallerPackage() string {
	const replayModeRecordModeCaller = 3
	c := caller.Get(replayModeRecordModeCaller)
	pkg := strings.SplitN(c, ".", 2)[0]
	return path.Base(pkg)
}

// ReplayMode returns a precondition that WPR is started in replay mode using
// the given archive as data file of the package and Chrome is logged in and
// redirects its traffic through WPR.
//
// The precondition is keyed by pkg and archive. Tests of the same package
// and the same |archive| would use the same precondition instance and save
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
func ReplayMode(archive string) testing.Precondition {
	return getOrCreatePrecondition(getCallerPackage(), archive, Replay)
}

// RecordMode returns a precondition similar to the ReplayMode above except
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
func RecordMode(archive string) testing.Precondition {
	return getOrCreatePrecondition(getCallerPackage(), archive, Record)
}

type preconditionImpl interface {
	Prepare(ctx context.Context, s *testing.State) interface{}
	Close(ctx context.Context, s *testing.State)
}

// preImpl implements both testing.Precondition and testing.preconditionImpl.
type preImpl struct {
	// Data for testing.Precondition.
	name    string
	timeout time.Duration

	// mode in which WPR runs.
	mode Mode
	// archive represents the path that WPR replays from or records to.
	archive string

	// WPR instance.
	wpr *WPR
	// Cancel function of the context that WPR process runs in.
	cancel context.CancelFunc

	// Chrome instance that runs with WPR Chrome options.
	cr *chrome.Chrome
}

func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

func (p *preImpl) Prepare(ctx context.Context, s *testing.State) interface{} {
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
		case Replay:
			archive = s.DataPath(p.archive)
		case Record:
			archive = p.archive
		default:
			s.Fatal("Unknown WPR mode: ", p.mode)
		}

		// Use context.Background() to create WPR instance because the WPR process
		// needs live beyond the |ctx| associated with Prepare stage.
		ctx, cancel := context.WithCancel(context.Background()) // NOLINT: WPR process needs to persist across multiple tests

		var err error
		if p.wpr, err = New(ctx, p.mode, archive); err != nil {
			s.Fatal("Failed to start WPR: ", err)
		}
		p.cancel = cancel
	}()

	testing.ContextLogf(ctx, "Starting Chrome with WPR at ports %d and %d",
		p.wpr.HTTPPort, p.wpr.HTTPSPort)
	var err error
	p.cr, err = chrome.New(ctx, p.wpr.ChromeOptions...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	chrome.Lock()

	return p.cr
}

func (p *preImpl) Close(ctx context.Context, s *testing.State) {
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

	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
}

// preMapKey holds variations that a preImpl instance could have.
type preMapKey struct {
	mode    Mode
	pkg     string
	archive string
}

func (k *preMapKey) String() string {
	return k.mode.String() + "_" + k.pkg + "_" + k.archive
}

// preMap is a map to track preImpl instances keyed by caller package name and
// WPR params.
var preMap = make(map[preMapKey]*preImpl)

// getOrCreatePrecondition gets existing instance of precondition that matches
// the given params and creates one if none of the existing instances matches.
func getOrCreatePrecondition(pkg, archive string, mode Mode) *preImpl {
	k := preMapKey{
		mode:    mode,
		pkg:     pkg,
		archive: archive,
	}
	if pre, ok := preMap[k]; ok {
		return pre
	}

	pre := &preImpl{
		name:    "wpr_" + k.String(),
		timeout: resetTimeout + wprTimeout + chrome.LoginTimeout,
		mode:    mode,
		archive: archive,
	}
	preMap[k] = pre
	return pre
}

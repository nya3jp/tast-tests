// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// wprTimeout is the time to wait for wpr sockets.
const wprTimeout = 10 * time.Second

// ReplayMode returns a precondition that wpr is started in replay mode using
// the given archive as data file of the package and chrome is logged in and
// redirects its traffic through wpr.
//
// The precondition is keyed by |pkg| and |archive|. Tests of the same package
// and the same |archive| would use the same precondition instance and save
// the time to start wpr and chrome.
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
//			Pre: wpr.ReplayMode("your_package", "example_wpr_archive.wprgo"),
//		})
//	}
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		// |cr| is a logged-in chrome with net traffic redirected to wpr.
//		cr := s.PreValue().(*chrome.Chrome)
//		...
//	}
func ReplayMode(pkg, archive string) testing.Precondition {
	return newPrecondition(pkg, archive, Replay)
}

// RecordMode returns a precondition similar to the ReplayMode above except
// wpr runs in record mode and all sites accessed by chrome are recorded
// in the given archive path on the device.
//
// Example usage:
//
//	func init() {
//		testing.AddTest(&testing.Test{
//			Func: DoSomething
//			...
//			Pre: wpr.RecordMode("your_package", "/tmp/example_wpr_archive.wprgo"),
//		})
//	}
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		// |cr| is a logged-in chrome with net traffic redirected through wpr
//    // and recorded.
//		cr := s.PreValue().(*chrome.Chrome)
//		...
//	}
func RecordMode(pkg, archive string) testing.Precondition {
	return newPrecondition(pkg, archive, Record)
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

	// Data to start wpr.
	mode    Mode
	archive string

	// WPR instance.
	wpr *WPR
	// Cancel function of the context that wpr process runs in.
	cancel context.CancelFunc

	// Chrome precondition that runs chrome with wpr chrome options.
	crPre testing.Precondition
}

func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

func (p *preImpl) Prepare(ctx context.Context, s *testing.State) interface{} {
	if p.wpr != nil {
		err := func() error {
			ctx, cancel := context.WithTimeout(ctx, wprTimeout)
			defer cancel()
			ctx, st := timing.Start(ctx, "reset_"+p.name)
			defer st.End()
			if err := p.wpr.Wait(ctx); err != nil {
				return errors.Wrap(err, "existing wpr is unusable")
			}
			return nil
		}()
		if err == nil {
			return p.crPre.(preconditionImpl).Prepare(ctx, s)
		}

		p.Close(ctx, s)
	}

	func() {
		params := &Params{Mode: p.mode}
		switch p.mode {
		case Replay:
			params.WPRArchivePath = s.DataPath(p.archive)
		case Record:
			params.WPRArchivePath = p.archive
		default:
			s.Fatal("Unknown wpr mode: ", p.mode)
		}

		// Use context.Background() to create wpr instance because the wpr process
		// needs live beyond the |ctx| associated with Prepare stage.
		ctx, cancel := context.WithCancel(context.Background())

		var err error
		if p.wpr, err = New(ctx, params); err != nil {
			s.Fatal("Failed to start wpr: ", err)
		}
		p.cancel = cancel
	}()

	// Chrome can start before WPR is ready because it will not need it until
	// we start opening tabs.
	testing.ContextLogf(ctx, "Starting Chrome with WPR at ports %d and %d",
		p.wpr.HTTPPort, p.wpr.HTTPSPort)
	p.crPre = chrome.NewPrecondition("wpr_pre_chrome", p.wpr.ChromeOptions...)
	cr := p.crPre.(preconditionImpl).Prepare(ctx, s)

	func() {
		ctx, cancel := context.WithTimeout(ctx, wprTimeout)
		defer cancel()
		if err := p.wpr.Wait(ctx); err != nil {
			s.Fatal("Failed to wait for wpr: ", err)
		}
	}()

	return cr
}

func (p *preImpl) Close(ctx context.Context, s *testing.State) {
	p.crPre.(preconditionImpl).Close(ctx, s)

	if p.wpr != nil {
		if err := p.wpr.Close(); err != nil {
			s.Fatal("Failed to stop wpr: ", err)
		}
		p.wpr = nil
	}

	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
}

// preMap is a map to track preImpl instances keyed by caller package name and
// wpr params.
var preMap = make(map[string]*preImpl)

func newPrecondition(pkg, archive string, mode Mode) *preImpl {
	hash := sha256.Sum256([]byte(fmt.Sprintf(
		"pkg:%s, archive:%s, mode:%d", pkg, archive, mode)))
	k := hex.EncodeToString(hash[:])
	if pre, ok := preMap[k]; ok {
		return pre
	}

	pre := &preImpl{
		name:    "wpr_" + k,
		timeout: wprTimeout + chrome.LoginTimeout,
		mode:    mode,
		archive: archive,
	}
	preMap[k] = pre
	return pre
}

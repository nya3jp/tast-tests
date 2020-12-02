// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpr

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// preKeepStateImpl implements testing.Precondition.
type preKeepStateImpl struct {
	name    string
	timeout time.Duration
	cr      *chrome.Chrome
}

func (p *preKeepStateImpl) String() string         { return p.name }
func (p *preKeepStateImpl) Timeout() time.Duration { return p.timeout }

func (p *preKeepStateImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	httpAddr := s.RequiredVar("wpr_http_addr")
	httpsAddr := s.RequiredVar("wpr_https_addr")
	if err := waitForServerSocket(ctx, httpAddr, nil); err != nil {
		s.Fatalf("Cannot connect to WPR at %s: %v", httpAddr, err)
	}
	testing.ContextLog(ctx, "WPR HTTP socket is up at ", httpAddr)

	if p.cr != nil {
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
			s.Log("Reusing existing Chrome session")
			return p.cr
		}

		s.Log("Failed to reuse existing Chrome session: ", err)
		p.Close(ctx, s)
	}

	const spkiList = "PhrPvGIaAMmd29hj8BCZOq096yj7uMpRNHpn5PDxI6I="
	var (
		resolverRules     = fmt.Sprintf("MAP *:80 %s,MAP *:443 %s,EXCLUDE localhost", httpAddr, httpsAddr)
		resolverRulesFlag = fmt.Sprintf("--host-resolver-rules=%q", resolverRules)
		spkiListFlag      = fmt.Sprintf("--ignore-certificate-errors-spki-list=%s", spkiList)
		args              = []string{resolverRulesFlag, spkiListFlag}
		err               error
	)

	testing.ContextLogf(ctx, "Starting Chrome with remote WPR at addrs %s and %s", httpAddr, httpsAddr)
	p.cr, err = chrome.New(ctx, chrome.KeepState(), chrome.ExtraArgs(args...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	chrome.Lock()

	return p.cr
}

func (p *preKeepStateImpl) Close(ctx context.Context, s *testing.PreState) {
	if p.cr != nil {
		chrome.Unlock()
		if err := p.cr.Close(ctx); err != nil {
			s.Fatal("Failed to close Chrome connection: ", err)
		}
		p.cr = nil
	}
}

var preKeepState = &preKeepStateImpl{
	name:    "wpr_keep_state",
	timeout: resetTimeout + chrome.LoginTimeout,
}

// RecordKeepState returns a precondition that Chrome is logged in, preserving the state
// and redirects its traffic through a remote WPR.
func RecordKeepState() testing.Precondition {
	return preKeepState
}

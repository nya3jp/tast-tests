// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chrome implements a library used for communication with Chrome.
package chrome

import (
	"context"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/testing"
)

// Target contains information about an available debugging target to which a connection can be established.
type Target struct {
	// URL contains the URL of the resource currently loaded by the target.
	URL string
	// The type of the target. It's obtained from target.Info.Type.
	Type string
}

func newTarget(t *target.Info) *Target {
	return &Target{URL: t.URL, Type: t.Type}
}

// TargetMatcher is a caller-provided function that matches targets with specific characteristics.
type TargetMatcher func(t *Target) bool

// MatchTargetURL returns a TargetMatcher that matches targets with the supplied URL.
func MatchTargetURL(url string) TargetMatcher {
	return func(t *Target) bool { return t.URL == url }
}

// FindTarget iterates through all available targets and returns a connection to the
// first one that is matched by tm. It polls until the target is found or ctx's deadline expires.
// An error is returned if no target is found or tm matches multiple targets.
func FindTarget(ctx context.Context, d *cdputil.Session, tm TargetMatcher) (*target.Info, error) {
	var errNoMatch = errors.New("no targets matched")

	var all, matched []*target.Info
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		all, err = d.FindTargets(ctx, nil)
		if err != nil {
			return err
		}
		matched = []*target.Info{}
		for _, t := range all {
			if tm(newTarget(t)) {
				matched = append(matched, t)
			}
		}
		if len(matched) == 0 {
			return errNoMatch
		}
		return nil
	}, loginPollOpts); err != nil && err != errNoMatch {
		return nil, err
	}

	if len(matched) != 1 {
		testing.ContextLogf(ctx, "%d targets matched while unique match was expected. Existing targets:", len(matched))
		for _, t := range all {
			testing.ContextLogf(ctx, "  %+v", newTarget(t))
		}
		return nil, errors.Errorf("%d targets found", len(matched))
	}
	return matched[0], nil
}

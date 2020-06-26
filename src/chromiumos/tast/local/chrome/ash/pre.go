// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type preImpl struct {
	crPre      testing.Precondition
	numApps    int
	extDirBase string
	prepared   bool
}

var dummyApps100Pre = NewDummyAppPrecondition("dummy_apps", 100, chrome.NewPrecondition, false)
var dummyApps100PreSkiaRenderer = NewDummyAppPrecondition("dummy_apps", 100, chrome.NewPrecondition, true)

// NewDummyAppPrecondition creates a Precondition with numApps number of dummy apps, wrapping the Precondition
// created by innerPre.
func NewDummyAppPrecondition(name string, numApps int, innerPre func(name string, opts ...chrome.Option) testing.Precondition, skiaRenderer bool) *preImpl {
	name = fmt.Sprintf("%s_%d", name, numApps)
	tmpDir, err := ioutil.TempDir("", name)
	if err != nil {
		panic(err)
	}
	opts := make([]chrome.Option, 0, numApps)
	for i := 0; i < numApps; i++ {
		opts = append(opts, chrome.UnpackedExtension(filepath.Join(tmpDir, fmt.Sprintf("dummy_%d", i))))
	}
	if skiaRenderer {
		opts = append(opts, chrome.ExtraArgs("--enable-features=UseSkiaRenderer"))
	}
	crPre := innerPre(name, opts...)
	return &preImpl{crPre: crPre, numApps: numApps, extDirBase: tmpDir, prepared: false}
}

// LoggedInWith100DummyApps returns the precondition that Chrome is already
// logged in and 100 dummy applications (extensions) are installed. PreValue for
// the test with this precondition is an instance of *chrome.Chrome.
func LoggedInWith100DummyApps() testing.Precondition {
	return dummyApps100Pre
}

// LoggedInWith100DummyAppsWithSkiaRenderer creates a precondition similar
// to LoggedInWith100DummyApps with the added feature SkiaRenderer enabled.
func LoggedInWith100DummyAppsWithSkiaRenderer() testing.Precondition {
	return dummyApps100PreSkiaRenderer
}

func (p *preImpl) String() string         { return p.crPre.String() }
func (p *preImpl) Timeout() time.Duration { return p.crPre.Timeout() }

func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	if !p.prepared {
		_, err := PrepareDummyApps(p.extDirBase, p.numApps)
		if err != nil {
			s.Fatal("Failed to prepare dummy apps: ", err)
		}
		p.prepared = true
	}
	return p.crPre.Prepare(ctx, s)
}

func (p *preImpl) Close(ctx context.Context, s *testing.PreState) {
	p.crPre.Close(ctx, s)
	if err := os.RemoveAll(p.extDirBase); err != nil {
		s.Fatal("Failed to cleanup ", p.extDirBase, " ", err)
	}
}

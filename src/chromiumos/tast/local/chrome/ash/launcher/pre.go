// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
)

const resetTimeout = 15 * time.Second

type preconditionImpl interface {
	Prepare(ctx context.Context, s *testing.State) interface{}
	Close(ctx context.Context, s *testing.State)
}

type preImpl struct {
	crPre      testing.Precondition
	numApps    int
	extDirBase string
	prepared   bool
}

var dummyApps100PreChromeOS = newPrecondition("dummy_apps", 100, lacros.ChromeTypeChromeOS)
var dummyApps100PreLacros = newPrecondition("dummy_apps", 100, lacros.ChromeTypeLacros)

func newPrecondition(name string, numApps int, crt lacros.ChromeType) *preImpl {
	name = fmt.Sprintf("%s_%d", name, numApps)
	tmpDir, err := ioutil.TempDir("", name)
	if err != nil {
		panic(err)
	}
	opts := make([]chrome.Option, 0, numApps)
	for i := 0; i < numApps; i++ {
		opts = append(opts, chrome.UnpackedExtension(filepath.Join(tmpDir, fmt.Sprintf("dummy_%d", i))))
	}
	var crPre testing.Precondition
	if crt == lacros.ChromeTypeLacros {
		crPre = launcher.StartedByDataWithChromeOSOptions(name, opts...)

	} else {
		crPre = chrome.NewPrecondition(name, opts...)
	}
	return &preImpl{crPre: crPre, numApps: numApps, extDirBase: tmpDir, prepared: false}
}

// LoggedInWith100DummyApps returns the precondition that Chrome is already
// logged in and 100 dummy applications (extensions) are installed. PreValue for
// the test with this precondition is an instance of *chrome.Chrome.
func LoggedInWith100DummyApps(crt lacros.ChromeType) testing.Precondition {
	if crt == lacros.ChromeTypeLacros {
		return dummyApps100PreLacros
	}
	return dummyApps100PreChromeOS
}

func (p *preImpl) String() string         { return p.crPre.String() }
func (p *preImpl) Timeout() time.Duration { return p.crPre.Timeout() }

func (p *preImpl) Prepare(ctx context.Context, s *testing.State) interface{} {
	if !p.prepared {
		_, err := PrepareDummyApps(p.extDirBase, p.numApps)
		if err != nil {
			s.Fatal("Failed to prepare dummy apps: ", err)
		}
		p.prepared = true
	}
	return p.crPre.(preconditionImpl).Prepare(ctx, s)
}

func (p *preImpl) Close(ctx context.Context, s *testing.State) {
	p.crPre.(preconditionImpl).Close(ctx, s)
	if err := os.RemoveAll(p.extDirBase); err != nil {
		s.Fatal("Failed to cleanup ", p.extDirBase, " ", err)
	}
}

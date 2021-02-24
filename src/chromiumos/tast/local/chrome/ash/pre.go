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

// NewFakeAppPrecondition creates a Precondition with numApps number of fake apps, wrapping the Precondition
// created by innerPre.
// TODO(crbug.com/1155613): deprecated. Should use fixtures.
func NewFakeAppPrecondition(name string, numApps int, innerPre func(name string, opts ...chrome.Option) testing.Precondition, skiaRenderer bool) *preImpl {
	name = fmt.Sprintf("%s_%d", name, numApps)
	tmpDir, err := ioutil.TempDir("", name)
	if err != nil {
		panic(err)
	}
	opts := make([]chrome.Option, 0, numApps)
	for i := 0; i < numApps; i++ {
		opts = append(opts, chrome.UnpackedExtension(filepath.Join(tmpDir, fmt.Sprintf("fake_%d", i))))
	}
	if skiaRenderer {
		name = name + "_skia_renderer"
		opts = append(opts, chrome.EnableFeatures("UseSkiaRenderer"))
	}
	crPre := innerPre(name, opts...)
	return &preImpl{crPre: crPre, numApps: numApps, extDirBase: tmpDir, prepared: false}
}

func (p *preImpl) String() string         { return p.crPre.String() }
func (p *preImpl) Timeout() time.Duration { return p.crPre.Timeout() }

func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	if !p.prepared {
		_, err := PrepareFakeApps(p.extDirBase, p.numApps, nil)
		if err != nil {
			s.Fatal("Failed to prepare fake apps: ", err)
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

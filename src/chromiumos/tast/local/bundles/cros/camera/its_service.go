// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"io/ioutil"
	"os"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	cameraboxpb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			cameraboxpb.RegisterITSServiceServer(srv, &ITSService{s: s})
		},
	})
}

// ITSService implements tast.cros.camerabox.ITSService.
type ITSService struct {
	s *testing.ServiceState

	cr     *chrome.Chrome
	a      *arc.ARC
	outDir string
}

func (its *ITSService) SetUp(ctx context.Context, req *empty.Empty) (_ *empty.Empty, retErr error) {
	if its.cr != nil {
		return nil, errors.New("DUT for running ITS is already set up")
	}

	// Set up chrome.
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.RestrictARCCPU(),
		chrome.KeepState(), chrome.ExtraArgs("--disable-arc-data-wipe", "--ignore-arcvm-dev-conf"))
	if err != nil {
		testing.ContextLog(ctx, "Error setting up chrome: ", err)
		return nil, err
	}
	defer func() {
		if retErr != nil {
			cr.Close(ctx)
		}
	}()

	// Set up ARC++.
	outDir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a temp dir")
	}
	defer func() {
		if retErr != nil {
			os.RemoveAll(outDir)
		}
	}()

	a, err := arc.New(ctx, outDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start ARC")
	}
	defer func() {
		if retErr != nil {
			a.Close()
		}
	}()

	its.cr = cr
	its.a = a
	its.outDir = outDir
	return &empty.Empty{}, nil
}

func (its *ITSService) TearDown(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if its.cr == nil {
		return nil, errors.New("cannot call ITS tear down on DUT before setting up")
	}
	var firstErr error
	for _, cleanup := range [](func() error){
		func() error { return its.a.Close() },
		func() error { return os.RemoveAll(its.outDir) },
		func() error { return its.cr.Close(ctx) },
	} {
		if err := cleanup(); err != nil {
			if firstErr == nil {
				firstErr = errors.Wrap(err, "failed to turn tear down process")
			} else {
				testing.ContextLog(ctx, "Failed to run tear down process: ", err)
			}
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return &empty.Empty{}, nil
}

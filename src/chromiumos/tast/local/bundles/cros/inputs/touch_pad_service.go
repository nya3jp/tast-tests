// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/services/cros/inputs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			inputs.RegisterTouchpadServiceServer(srv, &TouchpadService{s: s})
		},
	})
}

// TouchpadService implements tast.cros.inputs.TouchpadService.
type TouchpadService struct {
	s         *testing.ServiceState
	cr        *chrome.Chrome
	devPathTP string
}

// NewChrome starts a new Chrome session, and logs in as a test user.
func (tp *TouchpadService) NewChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if tp.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, err
	}
	tp.cr = cr

	return &empty.Empty{}, nil
}

// FindPhysicalTouchpad iterates over devices, and returns the path for a physical touch pad if one exists.
func (tp *TouchpadService) FindPhysicalTouchpad(ctx context.Context, req *empty.Empty) (*inputs.FindPhysicalTouchpadResponse, error) {

	foundTP, path, err := input.FindPhysicalTrackpad(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find device path for the touchpad")
	} else if !foundTP {
		return nil, errors.New("no phsyical trackpad found")
	} else {
		tp.devPathTP = path
		return &inputs.FindPhysicalTouchpadResponse{Path: path}, nil
	}
}

// TouchpadSwipe performs a swipe on a physical touch pad.
func (tp *TouchpadService) TouchpadSwipe(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {

	// Prepare trackpad.
	tpd, err := input.Trackpad(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a trackpad device")
	}
	defer tpd.Close()

	tpw, err := tpd.NewMultiTouchWriter(4)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a multi touch writer")
	}
	defer tpw.Close()

	// Performs a four finger horizontal scroll on the trackpad. The vertical location is always vertically
	// centered on the trackpad. The fingers are spaced horizontally on the trackpad by 1/16th of the trackpad
	// width.
	fingerSpacing := tpd.Width() / 16
	doTrackpadFourFingerSwipeScroll := func(ctx context.Context, x0, x1 input.TouchCoord) error {
		y := tpd.Height() / 2
		const t = time.Second
		return tpw.Swipe(ctx, x0, y, x1, y, fingerSpacing, 4, t)
	}

	fingerDistance := fingerSpacing * 4

	// Do a big swipe going right.
	if err := doTrackpadFourFingerSwipeScroll(ctx, 0, tpd.Width()-fingerDistance); err != nil {
		return nil, errors.Wrap(err, "failed to perform four finger scroll")
	}

	if err := tpw.End(); err != nil {
		return nil, errors.Wrap(err, "failed to finish trackpad scroll")
	}

	return &empty.Empty{}, nil
}

// CloseChrome closes a Chrome session and cleans up the resources obtained by NewChrome.
// Also, CloseChrome must be called after, not prior to, NewChrome.
func (tp *TouchpadService) CloseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if tp.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	err := tp.cr.Close(ctx)
	tp.cr = nil
	return &empty.Empty{}, err
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"bytes"
	"context"
	"regexp"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/testexec"
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
	s  *testing.ServiceState
	cr *chrome.Chrome
}

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

func (tp *TouchpadService) ReadEvtestTouchpad(ctx context.Context, req *inputs.ReadEvtestTouchpadRequest) (*inputs.ReadEvtestTouchpadResponse, error) {

	var detected bool

	ctxEvtest, cancelEvtest := context.WithTimeout(ctx, time.Duration(req.Duration)*time.Second)
	defer cancelEvtest()

	cmd := testexec.CommandContext(ctxEvtest, "evtest", "--grab", "/dev/input/event4")
	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to read touchpad")
	}
	if err := cmd.Wait(); err == nil {
		return nil, errors.New("evtest unexpectedly did not time out")
	} else if !errors.Is(err, context.DeadlineExceeded) {
		return nil, errors.Wrap(err, "evtest exited unexpectedly")
	}

	re := regexp.MustCompile(`\(ABS_PRESSURE\), value 0`)
	matches := re.FindAllSubmatch(stdout.Bytes(), -1)
	if matches != nil {
		detected = true
	}
	tp.s.Logf("Detected action on DUT's touch pad: %t", detected)

	return &inputs.ReadEvtestTouchpadResponse{TpEventDetected: detected}, nil
}

func (tp *TouchpadService) CloseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if tp.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	err := tp.cr.Close(ctx)
	tp.cr = nil
	return &empty.Empty{}, err
}

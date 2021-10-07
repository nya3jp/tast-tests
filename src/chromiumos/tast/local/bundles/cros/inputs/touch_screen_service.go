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
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	"chromiumos/tast/services/cros/inputs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			inputs.RegisterTouchscreenServiceServer(srv, &TouchscreenService{s: s})
		},
	})
}

// TouchscreenService implements tast.cros.inputs.TouchscreenService.
type TouchscreenService struct {
	s  *testing.ServiceState
	cr *chrome.Chrome
}

func (ts *TouchscreenService) NewChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if ts.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, err
	}
	ts.cr = cr
	return &empty.Empty{}, nil
}

func (ts *TouchscreenService) ReuseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if ts.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	cr, err := chrome.New(ctx, chrome.TryReuseSession())
	if err != nil {
		return nil, err
	}
	ts.cr = cr
	return &empty.Empty{}, nil
}

func (ts *TouchscreenService) TouchscreenTap(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {

	tconn, err := ts.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	// Prepare touchscreen. Note, the size of touchscreen might not be the same as
	// the display size.
	tsn, err := input.Touchscreen(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to access the touchscreen")
	}
	defer tsn.Close()

	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "no display")
	}

	touchWidth := tsn.Width()
	touchHeight := tsn.Height()

	displayWidth := float64(info.Bounds.Width)
	displayHeight := float64(info.Bounds.Height)

	pixelToTouchFactorX := float64(touchWidth) / displayWidth
	pixelToTouchFactorY := float64(touchHeight) / displayHeight

	centerX := displayWidth * pixelToTouchFactorX / 2
	centerY := displayHeight * pixelToTouchFactorY / 2

	stw, err := tsn.NewSingleTouchWriter()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a single touch writer")
	}
	defer stw.Close()

	// Values must be in "touchscreen coordinates", not pixel coordinates.
	stw.Move(input.TouchCoord(centerX), input.TouchCoord(centerY))
	stw.End()

	return &empty.Empty{}, nil
}

func (ts *TouchscreenService) ReadEvtestTouchscreen(ctx context.Context, req *inputs.ReadEvtestTouchscreenRequest) (*inputs.ReadEvtestTouchscreenResponse, error) {

	var detected bool

	ctxEvtest, cancelEvtest := context.WithTimeout(ctx, time.Duration(req.Duration)*time.Second)
	defer cancelEvtest()

	cmd := testexec.CommandContext(ctxEvtest, "evtest", "--grab", "/dev/input/event5")
	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to read touchscreen")
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
	ts.s.Logf("Detected action on DUT's touch screen: %t", detected)

	return &inputs.ReadEvtestTouchscreenResponse{TscreenEventDetected: detected}, nil
}

func (ts *TouchscreenService) CloseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if ts.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	err := ts.cr.Close(ctx)
	ts.cr = nil
	return &empty.Empty{}, err
}

// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			ui.RegisterAudioServiceServer(srv, &AudioService{s: s})
		},
	})
}

// AudioService implements tast.cros.ui.AudioService.
type AudioService struct {
	s  *testing.ServiceState
	cr *chrome.Chrome
}

// New logs into a Chrome session as a fake user. Close must be called later
// to clean up the associated resources.
func (as *AudioService) New(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if as.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, err
	}
	as.cr = cr
	return &empty.Empty{}, nil
}

// Close releases the resources obtained by New.
func (as *AudioService) Close(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if as.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	err := as.cr.Close(ctx)
	as.cr = nil
	return &empty.Empty{}, err
}

// OpenDirectoryAndFile performs launching filesapp and opening particular file
// in given directory.
func (as *AudioService) OpenDirectoryAndFile(ctx context.Context, req *ui.AudioServiceRequest) (*empty.Empty, error) {
	if as.cr == nil {
		return nil, errors.New("Chrome not available")
	}

	tconn, err := as.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	filesTitlePrefix := "Files - "
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch the Files App")
	}

	if err := files.OpenDir(req.DirectoryName, filesTitlePrefix+req.DirectoryName)(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to open %q folder in files app", req.DirectoryName)
	}

	if req.FileName != "" {
		if err := files.OpenFile(req.FileName)(ctx); err != nil {
			return nil, errors.Wrapf(err, "failed to open the audio file %q", req.FileName)
		}
	}

	return &empty.Empty{}, nil
}

// GenerateTestRawData generates test raw data file.
func (as *AudioService) GenerateTestRawData(ctx context.Context, req *ui.AudioServiceRequest) (*empty.Empty, error) {
	if as.cr == nil {
		return nil, errors.New("Chrome not available")
	}

	// Generate sine raw input file that lasts 30 seconds.
	rawFile := audio.TestRawData{
		Path:          req.FilePath,
		BitsPerSample: 16,
		Channels:      2,
		Rate:          48000,
		Frequencies:   []int{440, 440},
		Volume:        0.05,
		Duration:      int(req.DurationInSecs),
	}

	if err := audio.GenerateTestRawData(ctx, rawFile); err != nil {
		return nil, errors.Wrap(err, "failed to generate audio test data")
	}
	return &empty.Empty{}, nil
}

// ConvertRawToWav will convert raw data file to wav file format.
func (as *AudioService) ConvertRawToWav(ctx context.Context, req *ui.AudioServiceRequest) (*empty.Empty, error) {
	if as.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	if err := audio.ConvertRawToWav(ctx, req.FilePath, req.FileName, 48000, 2); err != nil {
		return nil, errors.Wrap(err, "failed to convert raw to wav")
	}
	return &empty.Empty{}, nil
}

// KeyboardAccel will create keyboard event and performs keyboard
// key press with Accel().
func (as *AudioService) KeyboardAccel(ctx context.Context, req *ui.AudioServiceRequest) (*empty.Empty, error) {
	if as.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find keyboard")
	}
	defer kb.Close()

	if err := kb.Accel(ctx, req.Expr); err != nil {
		return nil, errors.Wrapf(err, "failed to press %q using keyboard", req.Expr)
	}
	return &empty.Empty{}, nil
}

// AudioCrasSelectedOutputDevice will return selected audio device name
// and audio device type.
func (as *AudioService) AudioCrasSelectedOutputDevice(ctx context.Context, req *empty.Empty) (*ui.AudioServiceResponse, error) {
	if as.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	// Get Current active node.
	cras, err := audio.NewCras(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Cras object")
	}
	outDeviceName, outDeviceType, err := cras.SelectedOutputDevice(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the selected audio device")
	}
	return &ui.AudioServiceResponse{DeviceName: outDeviceName, DeviceType: outDeviceType}, nil
}

// VerifyFirstRunningDevice will check for audio routing device status.
func (as *AudioService) VerifyFirstRunningDevice(ctx context.Context, req *ui.AudioServiceRequest) (*empty.Empty, error) {
	if as.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
		if err != nil {
			return errors.Wrap(err, "failed to detect running output device")
		}

		if deviceName := req.Expr; deviceName != devName {
			return errors.Wrapf(err, "failed to route the audio through expected audio node: got %q; want %q", devName, deviceName)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return nil, errors.Wrap(err, "failed to check audio running device")
	}
	return &empty.Empty{}, nil
}

// SetActiveNodeByType will set the provided audio node as Active audio node.
func (as *AudioService) SetActiveNodeByType(ctx context.Context, req *ui.AudioServiceRequest) (*empty.Empty, error) {
	if as.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	var cras *audio.Cras
	if err := cras.SetActiveNodeByType(ctx, req.Expr); err != nil {
		return nil, errors.Wrapf(err, "failed to select active device %s", req.Expr)
	}
	return &empty.Empty{}, nil
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multimedia

import (
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf/audio"
	"chromiumos/tast/local/mtbf/service"
	"chromiumos/tast/services/mtbf/multimedia"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			multimedia.RegisterVolumeServiceServer(srv, &VolumeService{service.New(s)})
		},
	})
}

// An VolumeService implements the tast/services/mtbf/multimedia.VolumeService.
type VolumeService struct {
	service.Service
}

func (s *VolumeService) Get(ctx context.Context, _ *empty.Empty) (*multimedia.VolumeResponse, error) {
	conn, err := s.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}
	testing.ContextLog(ctx, "VolumeService: get volume")

	v, err := audio.GetOSVolume(ctx, conn)
	if err != nil {
		return nil, err
	}
	m, err := audio.IsOSVolumeMute(ctx, conn)
	if err != nil {
		return nil, err
	}

	return &multimedia.VolumeResponse{Value: int64(v), Mute: m}, nil
}

func (s *VolumeService) Set(ctx context.Context, req *multimedia.VolumeRequest) (*empty.Empty, error) {
	conn, err := s.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}
	testing.ContextLog(ctx, "VolumeService: set volume to ", req.Value)

	if err := audio.SetOSVolume(ctx, conn, int(req.Value)); err != nil {
		return nil, err
	}

	if req.Check {
		v, err := audio.GetOSVolume(ctx, conn)
		if err != nil || int64(v) != req.Value {
			return nil, mtbferrors.New(mtbferrors.AudioVolume, err, v, req.Value)
		}
	}
	return &empty.Empty{}, nil
}

func (s *VolumeService) setMute(ctx context.Context, mute bool) error {
	conn, err := s.TestAPIConn(ctx)
	if err != nil {
		return err
	}
	testing.ContextLog(ctx, "VolumeService: set mute to ", mute)

	return audio.SetOSVolumeMute(ctx, conn, mute)
}

func (s *VolumeService) Mute(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	return &empty.Empty{}, s.setMute(ctx, true)
}

func (s *VolumeService) Unmute(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	return &empty.Empty{}, s.setMute(ctx, false)
}

func (s *VolumeService) PressKey(ctx context.Context, req *multimedia.PressKeyRequest) (*multimedia.VolumeResponse, error) {
	conn, err := s.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	key, ok := multimedia.FnKey_name[int32(req.Key)]
	if !ok || req.Key == multimedia.FnKey_UNKNOWN {
		return nil, status.Error(codes.InvalidArgument, "unknown volume control key")
	}

	// init keyboard
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeGetKeyboard, err)
	}
	defer kb.Close()
	testing.ContextLog(ctx, "VolumeService: keyboard initiated")

	// get original volume
	v, err := audio.GetOSVolume(ctx, conn)
	if err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "VolumeService: press key ", key)
	if err = kb.Accel(ctx, key); err != nil {
		return nil, err
	}
	testing.Sleep(ctx, time.Second)

	// get new volume
	n, err := audio.GetOSVolume(ctx, conn)
	if err != nil {
		return nil, err
	}
	m, err := audio.IsOSVolumeMute(ctx, conn)
	if err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "VolumeService: volume changed from %d to %d", v, n)

	// verify if the volume changed
	if req.Check {
		switch req.Key {
		case multimedia.FnKey_F10: // louder
			if n != 100 && !(n > v) {
				return nil, mtbferrors.New(mtbferrors.AudioChgVol, nil, n, v+4)
			}
		case multimedia.FnKey_F9: // lower
			if n != 0 && !(n < v) {
				return nil, mtbferrors.New(mtbferrors.AudioChgVol, nil, n, v-4)
			}
		case multimedia.FnKey_F8: // mute
			if !m {
				return nil, mtbferrors.New(mtbferrors.AudioMute, nil)
			}
		}
	}

	return &multimedia.VolumeResponse{Value: int64(v), Mute: m}, nil
}

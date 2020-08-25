// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multimedia

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf/audio"
	"chromiumos/tast/local/mtbf/mtbfutil/common"
	"chromiumos/tast/local/mtbf/service"
	"chromiumos/tast/services/mtbf/multimedia"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			multimedia.RegisterAudioPlayerServer(srv, &AudioPlayer{service.New(s)})
		},
	})
}

var audioPlayer = apps.App{
	ID:   "cjbfomnbifhcdnihkgipgfcihmgjfhbf",
	Name: "Audio Player",
}

// An AudioPlayer implements the tast/services/mtbf/multimedia.AudioPlayerServer.
type AudioPlayer struct {
	service.Service
}

// OpenInDownloads launches the Files app, opening the file by the
// relative path insides the Downloads directory.
func (s *AudioPlayer) OpenInDownloads(ctx context.Context, req *multimedia.FileRequest) (*empty.Empty, error) {

	// check if the file exists
	path := filepath.Join(filesapp.DownloadPath, req.Filepath)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, mtbferrors.New(mtbferrors.GRPCFileNotFound, nil, path)
	}

	conn, err := s.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}
	testing.ContextLog(ctx, "AudioPlayer: open ", path)

	app, err := filesapp.Launch(ctx, conn)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeOpenFileApps, err)
	}
	testing.ContextLog(ctx, "AudioPlayer: Files app launched")

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeGetKeyboard, err)
	}
	defer kb.Close()
	testing.ContextLog(ctx, "AudioPlayer: keyboard initiated")

	testing.Sleep(ctx, time.Second) // make sure the app already shows up
	if err = app.OpenDownloads(ctx); err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeOpenFolder, err, "Downloads")
	}

	// select the directories and the file by the path
	for _, name := range strings.Split(strings.TrimPrefix(path, filesapp.DownloadPath), "/") {
		testing.Sleep(ctx, 2*time.Second) // make sure UI is stable before clicking
		if err = app.WaitForFile(ctx, name, 10*time.Second); err != nil {
			return nil, mtbferrors.New(mtbferrors.ChromeClickItem, err, req.Filepath)
		}

		if err = app.SelectFile(ctx, name); err != nil {
			return nil, mtbferrors.New(mtbferrors.ChromeRenderTime, err, req.Filepath)
		}
		testing.Sleep(ctx, 300*time.Millisecond)

		if err = kb.Accel(ctx, "Enter"); err != nil {
			return nil, mtbferrors.New(mtbferrors.ChromeKeyPress, err, "Enter")
		}
	}

	// wait for the audio player shows up
	params := ui.FindParams{
		Role: ui.RoleTypeWindow,
		Name: audioPlayer.Name,
	}
	if err = ui.WaitUntilExists(ctx, conn, params, time.Minute); err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeOpenAudioPlayer, err)
	}

	return &empty.Empty{}, nil
}

// Focus makes the window switched to the audio player.
func (s *AudioPlayer) Focus(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	conn, err := s.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}
	testing.ContextLog(ctx, "AudioPlayer: switch to the app")

	// return if the window is already active
	window, err := ash.FindWindow(ctx, conn, func(w *ash.Window) bool { return w.Title == audioPlayer.Name })
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Audio Player is not launched")
	}
	if window.IsActive {
		return &empty.Empty{}, nil
	}

	// make sure the ChromeOS shelf is visible and switch to the app by clicking
	// the icon in the shelf
	if err = common.ShelfVisible(ctx, conn, func() error {
		params := ui.FindParams{
			Role:      ui.RoleTypeButton,
			Name:      audioPlayer.Name,
			ClassName: "ash/ShelfAppButton",
		}
		return common.ClickElement(ctx, conn, params)
	}); err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeClickItem, err, audioPlayer.Name)
	}
	return &empty.Empty{}, nil
}

// Close closes the Audio Player app.
func (s *AudioPlayer) Close(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	conn, err := s.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}
	testing.ContextLog(ctx, "AudioPlayer: close audio player app")

	if err = apps.Close(ctx, conn, audioPlayer.ID); err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeCloseApp, err, "Audio Player")
	}
	return &empty.Empty{}, nil
}

// CloseAll closes the Audio Player and the Files app.
func (s *AudioPlayer) CloseAll(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	conn, err := s.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}
	testing.ContextLog(ctx, "AudioPlayer: close audio player and Files app")

	for _, app := range []apps.App{apps.Files, audioPlayer} {
		if err = apps.Close(ctx, conn, app.ID); err != nil {
			return nil, mtbferrors.New(mtbferrors.ChromeCloseApp, err, app.Name)
		}
	}
	return &empty.Empty{}, nil
}

func (s *AudioPlayer) Play(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	conn, err := s.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}
	testing.ContextLog(ctx, "AudioPlayer: wait and click play button")

	params := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "Play",
	}
	if err = ui.WaitUntilExists(ctx, conn, params, time.Minute); err != nil {
		return nil, mtbferrors.New(mtbferrors.AudioWaitPlayButton, err)
	}
	if err = common.ClickElement(ctx, conn, params); err != nil {
		return nil, mtbferrors.New(mtbferrors.AudioClickPlayButton, err)
	}

	return &empty.Empty{}, nil
}

func (s *AudioPlayer) Pause(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	conn, err := s.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}
	testing.ContextLog(ctx, "AudioPlayer: wait and click pause button")

	params := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "Pause",
	}
	if err = ui.WaitUntilExists(ctx, conn, params, time.Minute); err != nil {
		return nil, mtbferrors.New(mtbferrors.AudioWaitPauseButton, err)
	}
	if err = common.ClickElement(ctx, conn, params); err != nil {
		return nil, mtbferrors.New(mtbferrors.AudioClickPauseButton, err)
	}

	return nil, nil
}

func (s *AudioPlayer) CurrentTime(ctx context.Context, _ *empty.Empty) (*multimedia.TimeResponse, error) {
	conn, err := s.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	t, err := audio.GetAudioPlayingTime(ctx, conn)
	if err != nil {
		return nil, err
	}

	return &multimedia.TimeResponse{
		CurrentTime: int64(t),
	}, nil
}

func (s *AudioPlayer) checkPlaying(ctx context.Context, seconds int64) (old, current int, err error) {
	if seconds < 1 {
		return 0, 0, status.Errorf(codes.InvalidArgument, "timeout set to %d seconds", seconds)
	}

	conn, err := s.TestAPIConn(ctx)
	if err != nil {
		return 0, 0, err
	}

	if old, err = audio.GetAudioPlayingTime(ctx, conn); err != nil {
		return
	}

	if err = testing.Sleep(ctx, time.Duration(seconds)*time.Second); err != nil {
		return 0, 0, mtbferrors.New(mtbferrors.ChromeSleep, err)
	}

	current, err = audio.GetAudioPlayingTime(ctx, conn)
	return
}

// MustPlaying checks if the player is playing the audio, retunring an error
// if it's not.
func (s *AudioPlayer) MustPlaying(ctx context.Context, req *multimedia.TimeoutRequest) (*empty.Empty, error) {
	old, current, err := s.checkPlaying(ctx, req.Seconds)
	if err != nil {
		return nil, err
	}

	if old == current {
		return nil, mtbferrors.New(mtbferrors.AudioPlayFwd, nil, current, current, req.Seconds)
	}
	return &empty.Empty{}, nil
}

// MustPaused checks if the player is paused, returning an error if it's not.
func (s *AudioPlayer) MustPaused(ctx context.Context, req *multimedia.TimeoutRequest) (*empty.Empty, error) {
	old, current, err := s.checkPlaying(ctx, req.Seconds)
	if err != nil {
		return nil, err
	}

	if old != current {
		return nil, mtbferrors.New(mtbferrors.AudioPause, nil, req.Seconds, current, old)
	}
	return &empty.Empty{}, nil
}

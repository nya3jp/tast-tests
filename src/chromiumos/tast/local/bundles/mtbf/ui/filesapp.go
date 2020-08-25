// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/golang/protobuf/ptypes/timestamp"
	"google.golang.org/grpc"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf/service"
	pb "chromiumos/tast/services/mtbf/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterFilesAppServer(srv, &FilesApp{service.New(s)})
		},
	})
}

// A FilesApp implements the tast/services/mtbf/ui.FilesAppServer interface.
type FilesApp struct {
	service.Service
}

// Close closes the Files App.
func (s *FilesApp) Close(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	conn, err := s.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}
	testing.ContextLog(ctx, "FilesApp: close app")

	if err = apps.Close(ctx, conn, apps.Files.ID); err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeCloseApp, err, apps.Files.Name)
	}
	return &empty.Empty{}, nil
}

// SelectInDownloads launches the Files App, selecting a file by the relative
// path insides the Downloads directory. It opens the file if the request's
// Open field set to true.
func (s *FilesApp) SelectInDownloads(ctx context.Context, req *pb.FileRequest) (*empty.Empty, error) {

	// check if the file exists
	path := filepath.Join(filesapp.DownloadPath, req.Path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, mtbferrors.New(mtbferrors.GRPCFileNotFound, nil, path)
	}

	conn, err := s.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}
	testing.ContextLog(ctx, "FilesApp: select ", path)

	app, err := filesapp.Launch(ctx, conn)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeOpenFileApps, err)
	}
	defer app.Root.Release(ctx)

	// init a keyboard, so we can get into a directory by pressing "Enter"
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeGetKeyboard, err)
	}
	defer kb.Close()

	testing.Sleep(ctx, time.Second) // make sure the app already shows up
	if err = app.OpenDownloads(ctx); err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeOpenFolder, err, "Downloads")
	}

	// select the directories and the file by the path
	names := strings.Split(strings.TrimPrefix(path, filesapp.DownloadPath), "/")
	for i, name := range names {
		testing.Sleep(ctx, 2*time.Second) // make sure UI is stable before clicking
		if err = app.WaitForFile(ctx, name, 10*time.Second); err != nil {
			return nil, mtbferrors.New(mtbferrors.ChromeClickItem, err, req.Path)
		}

		if err = app.SelectFile(ctx, name); err != nil {
			return nil, mtbferrors.New(mtbferrors.GRPCFileNotFound, err, req.Path)
		}
		testing.Sleep(ctx, 400*time.Millisecond)

		if i == len(names)-1 && !req.Open {
			break
		}

		if err = kb.Accel(ctx, "Enter"); err != nil {
			return nil, mtbferrors.New(mtbferrors.ChromeKeyPress, err, "Enter")
		}
	}

	return &empty.Empty{}, nil
}

// Stat returns the file information by the absolute path. It does NOT launch
// the Files App.
func (s *FilesApp) Stat(ctx context.Context, req *pb.FileRequest) (*pb.FileResponse, error) {
	testing.ContextLog(ctx, "FilesApp: stat ", req.Path)

	info, err := os.Stat(req.Path)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.GRPCFileNotFound, nil, req.Path)
	}

	utc := info.ModTime().UTC()
	return &pb.FileResponse{
		Name: info.Name(),
		Size: info.Size(),
		ModTime: &timestamp.Timestamp{
			Seconds: utc.Unix(),
			Nanos:   int32(utc.Nanosecond()),
		},
		IsDir: info.IsDir(),
	}, nil
}

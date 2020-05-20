// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"strings"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	mtbfFilesapp "chromiumos/tast/local/mtbf/ui/filesapp"
	"chromiumos/tast/local/ui/filesapp"
	"chromiumos/tast/services/mtbf/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			ui.RegisterFilesAppServiceServer(srv, &FilesAppService{chrome.SvcLoginReusePre{S: s}})
		},
	})
}

// FilesAppService implements tast.mtbf.ui.FilesAppService.
type FilesAppService struct {
	//Use embedded structure to get the implementation of LoginReusePre
	chrome.SvcLoginReusePre
}

// GetAllFiles gets all files from download folder
func (c *FilesAppService) GetAllFiles(ctx context.Context, req *ui.FoldersRequest) (*ui.FilesResponse, error) {

	testing.ContextLog(ctx, "FilesAppService - GetAllFiles called")

	err := c.PrePrepare(ctx) // prepare the Chrome instance just in case
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}

	if c.CR == nil {
		return nil, mtbferrors.New(mtbferrors.ChromeInst, nil)
	}

	conn, err := c.CR.TestAPIConn(ctx)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeTestConn, err)
	}

	files, err := mtbfFilesapp.Launch(ctx, conn)
	if err != nil {
		return nil, err
	}
	defer filesapp.Close(ctx, conn)

	testing.Sleep(ctx, 2*time.Second) // Wait for ui to update
	_, mtbfErr := mtbfFilesapp.FocusOnFilesApp(ctx, conn, files, "My files")
	if mtbfErr != nil {
		return nil, mtbfErr
	}

	if req != nil {
		for _, folder := range req.Folders {

			if err := mtbfFilesapp.WaitAndEnterElement(ctx, files, filesapp.RoleStaticText, folder); err != nil {
				return nil, mtbferrors.New(mtbferrors.ChromeClickItem, err, folder)
			}
			testing.Sleep(ctx, 2*time.Second) // Wait for ui to update
		}
	}

	// 1. To make sure it's visible in case of item not found,
	//    so we have to soft files by modified date in descending order.
	// 2. We have to use "mtbfErr *mtbferrors.MTBFError" but not "err error" here.
	if mtbfErr := mtbfFilesapp.SortFilesByModifiedDateInDescendingOrder(ctx, files); mtbfErr != nil {
		return nil, mtbfErr
	}

	fileListStr, mtbfErr := mtbfFilesapp.LogFilesUnderCurrentFolder(ctx, conn)
	if mtbfErr != nil {
		return nil, mtbfErr
	}
	return &ui.FilesResponse{Files: strings.Split(fileListStr, "\n")}, nil
}

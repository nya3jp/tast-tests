// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ti50

import (
	"context"
	"time"
	"path/filepath"

        "google.golang.org/grpc"

	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/remote/firmware/serial"
	common "chromiumos/tast/common/firmware/ti50"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/dutfs"
)

// RemoteAndreiBoard controls a labstation-connected AndreiBoard.
type RemoteAndreiBoard struct {
	*common.AndreiBoard
        spiFlash              string
	remoteWorkDir         string
	remoteImagesDir        string
	remoteSpiFlash        string
	dut                   *dut.DUT
	grpcConn	      *grpc.ClientConn
}

// ListRemoteUltraDebugTargets returns a possibly empty list of UD devices.
func ListRemoteUltraDebugTargets(ctx context.Context, dut *dut.DUT) ([]string, error) {
	cmd := dut.Conn().CommandContext(ctx, "find", "/dev/")

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return common.ParseUltraDebugTargets(string(output)), nil
}

// NewRemoteAndreiBoard creates a RemoteAndreiBoard.
// 
// dut should have UltraDebug drivers installed, used to drive the AndreiBoard.
// bufSize should be set to the max outstanding chars waiting to be read.
// spiFlash is the path on the localhost, it will be copied to the dut.
// readTimeout should be set to the max expected duration between char outputs.
//
// Example:
//   TODO
func NewRemoteAndreiBoard(dut *dut.DUT, grpcConn *grpc.ClientConn, targetDevice string, bufSize int, spiFlash string, readTimeout time.Duration) *RemoteAndreiBoard {
	opener := serial.NewRemotePortOpener(grpcConn, targetDevice, 115200, readTimeout)
	ab := common.NewAndreiBoard(bufSize, opener)

	return &RemoteAndreiBoard{AndreiBoard: ab, spiFlash: spiFlash, dut: dut, grpcConn: grpcConn}
}

// copyImageToDut copies image at the path on host to dut, returning its path on the dut.
func (a *RemoteAndreiBoard) copyImageToDut(ctx context.Context, image string) (string, error) {
     remoteFile := filepath.Join(a.remoteImagesDir, "image.signed") 
     _, err := linuxssh.PutFiles(ctx, a.dut.Conn(), map[string]string{image: remoteFile}, linuxssh.DereferenceSymlinks)
     if err != nil {
     	return "", err
     }
     return remoteFile, nil
}

// setupDutWorkDir creates a scratch directory on dut to hold spiflash and
// images for subsequent use.
func (a *RemoteAndreiBoard) setupDutWorkDir(ctx context.Context) error {
	if a.remoteWorkDir != "" {
		return nil
	}

	dutfsClient := dutfs.NewClient(a.grpcConn)

	workDir, err := dutfsClient.TempDir(ctx, "", "")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
		        dutfsClient.RemoveAll(ctx, workDir)
		}
	}()

	imageDir, err := dutfsClient.TempDir(ctx, workDir, "images")
	if err != nil {
		return err
	}

	utilsDir, err := dutfsClient.TempDir(ctx, workDir, "utils")
	if err != nil {
		return err
	}

        spiflash := filepath.Join(utilsDir, "spiflash")

        _, err = linuxssh.PutFiles(ctx, a.dut.Conn(), map[string]string{a.spiFlash: spiflash}, linuxssh.DereferenceSymlinks)
	if err != nil {
		return err
	}

	a.remoteSpiFlash = spiflash
	a.remoteWorkDir = workDir
	a.remoteImagesDir = imageDir

	return nil
}

// FlashImage flashes image at the specified path on localhost to the board.
func (a *RemoteAndreiBoard) FlashImage(ctx context.Context, image string) error {
	if a.spiFlash == "" {
		return errors.New("spiflash binary not provided")
	}

	cmd := testexec.CommandContext(ctx, "ls", a.spiFlash)
	if err := cmd.Run(); err != nil {
		return errors.New("spiflash not found: " + a.spiFlash)
	}

	cmd = testexec.CommandContext(ctx, "ls", image)
	if err := cmd.Run(); err != nil {
		return errors.New("Image not found: " + image)
	}

	if err := a.setupDutWorkDir(ctx); err != nil {
		return err
	}

	remoteImage, err := a.copyImageToDut(ctx, image)
	if err != nil {
		return err
	}

	dutCmd := a.dut.Conn().CommandContext(ctx, a.remoteSpiFlash, "--dauntless", "--tty=2", "--verbose", "-X", "--input="+remoteImage)

	return dutCmd.Run()
}

// Resets the board via spiflash, causing the image to reboot.
func (a *RemoteAndreiBoard) Reset(ctx context.Context) error {
	if a.spiFlash == "" {
		return errors.New("spiflash binary not provided")
	}

	if err := a.setupDutWorkDir(ctx); err != nil {
		return err
	}

	cmd := a.dut.Conn().CommandContext(ctx, a.remoteSpiFlash, "-d", "-j")
	return cmd.Run()
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ti50

import (
	"context"
	"path/filepath"
	"time"

	"google.golang.org/grpc"

	common "chromiumos/tast/common/firmware/ti50"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/remote/firmware/serial"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/ssh/linuxssh"
)

// RemoteAndreiboard controls a labstation-connected Andreiboard.
type RemoteAndreiboard struct {
	*common.Andreiboard
	spiFlash        string
	remoteWorkDir   string
	remoteImagesDir string
	remoteSpiFlash  string
	dut             *dut.DUT
	grpcConn        *grpc.ClientConn
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

// NewRemoteAndreiboard creates a RemoteAndreiboard.
//
// dut should have UltraDebug drivers installed, used to drive the Andreiboard.
// bufSize should be set to the max outstanding chars waiting to be read.
// spiFlash is the path on the localhost, it will be copied to the dut.
// readTimeout should be set to the max expected duration between char outputs.
//
// Example:
//   rpcClient, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
//   if err != nil {
//       s.Fatal("rpcDial: ", err)
//   }
//   defer rpcClient.Close(ctx)
//
//   board := NewRemoteAndreiboard(s.DUT(), rpcClient.Conn, "/path/to/UDTarget", 4096, "/path/to/spiflash" 2 * time.Second)
//   defer board.Close(ctx)
func NewRemoteAndreiboard(dut *dut.DUT, grpcConn *grpc.ClientConn, targetDevice string, bufSize int, spiFlash string, readTimeout time.Duration) *RemoteAndreiboard {
	serialServiceClient := pb.NewSerialPortServiceClient(grpcConn)
	opener := serial.NewRemotePortOpener(serialServiceClient, targetDevice, 115200, readTimeout)
	ab := common.NewAndreiboard(bufSize, opener, spiFlash)

	return &RemoteAndreiboard{Andreiboard: ab, dut: dut, grpcConn: grpcConn}
}

// copyImageToDut copies image at the path on host to dut, returning its path on the dut.
func (a *RemoteAndreiboard) copyImageToDut(ctx context.Context, image string) (string, error) {
	remoteFile := filepath.Join(a.remoteImagesDir, "image.signed")
	_, err := linuxssh.PutFiles(ctx, a.dut.Conn(), map[string]string{image: remoteFile}, linuxssh.DereferenceSymlinks)
	if err != nil {
		return "", err
	}
	return remoteFile, nil
}

// setupDutWorkDir creates a scratch directory on dut to hold spiflash and
// images for subsequent use.
func (a *RemoteAndreiboard) setupDutWorkDir(ctx context.Context) error {
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

	_, err = linuxssh.PutFiles(ctx, a.dut.Conn(), map[string]string{a.GetSpiFlash(): spiflash}, linuxssh.DereferenceSymlinks)
	if err != nil {
		return err
	}

	a.remoteWorkDir = workDir
	a.remoteSpiFlash = spiflash
	a.remoteImagesDir = imageDir

	return nil
}

// FlashImage flashes image at the specified path on localhost to the board.
func (a *RemoteAndreiboard) FlashImage(ctx context.Context, image string) error {
	if a.GetSpiFlash() == "" {
		return errors.New("spiflash binary not provided")
	}

	cmd := testexec.CommandContext(ctx, "ls", a.GetSpiFlash())
	if err := cmd.Run(); err != nil {
		return errors.New("spiflash not found: " + a.GetSpiFlash())
	}

	cmd = testexec.CommandContext(ctx, "ls", image)
	if err := cmd.Run(); err != nil {
		return errors.New("image not found: " + image)
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

// OpenTitanToolCommand runs an arbitrary OpenTitan tool command (without up-/downloading any files).
func (a *RemoteAndreiboard) OpenTitanToolCommand(ctx context.Context, cmd string, args ...string) (output []byte, err error) {
	return nil, errors.New("Unimplemented RemoteAndreiboard.OttCommand")
}

// Reset resets the board via spiflash, causing the image to reboot.
func (a *RemoteAndreiboard) Reset(ctx context.Context) error {
	if a.GetSpiFlash() == "" {
		return errors.New("spiflash binary not provided")
	}

	if err := a.setupDutWorkDir(ctx); err != nil {
		return err
	}

	cmd := a.dut.Conn().CommandContext(ctx, a.remoteSpiFlash, "-d", "-j")
	return cmd.Run()
}

// Close and free resources.
func (a *RemoteAndreiboard) Close(ctx context.Context) error {
	if a.remoteWorkDir != "" {
		dutfsClient := dutfs.NewClient(a.grpcConn)
		dutfsClient.RemoveAll(ctx, a.remoteWorkDir)
	}
	return a.Andreiboard.Close(ctx)
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ti50

import (
	"context"
	"io/ioutil"
	"strings"
	"time"

	"google.golang.org/grpc"

	common "chromiumos/tast/common/firmware/ti50"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/ti50/dutcontrol"
)

const (
	// A value that is big enough so that console data from the raw uart shouldn't have to be broken up to multiple messages in most cases.
	consoleDataLen = 1024
)

// DUTControlAndreiboard controls an Andreiboard through dutcontrol grpc..
type DUTControlAndreiboard struct {
	client dutcontrol.DutControlClient
	*common.Andreiboard
}

// NewDUTControlAndreiboard creates a DUTControlAndreiboard.
//
// grpcConn should be to a host running the dutcontrol service.
// bufSize should be set to the max outstanding chars waiting to be read.
// readTimeout should be set to the max expected duration between char outputs.
//
// Example:
// conn, err := grpc.DialContext(ctx, hostPort, grpc.WithInsecure())
// if err != nil {
//     return nil, err
// }
// defer conn.Close(ctx)
// board := NewDUTControlAndreiboard(conn, 4096, 200 * time.Millisecond)
// defer board.Close(ctx)
func NewDUTControlAndreiboard(grpcConn *grpc.ClientConn, bufSize int, readTimeout time.Duration) *DUTControlAndreiboard {
	dutControlClient := dutcontrol.NewDutControlClient(grpcConn)
	opener := &DUTControlRawUARTPortOpener{
		Client:      dutControlClient,
		Uart:        ConsoleUart,
		Baud:        ConsoleBaud,
		DataLen:     consoleDataLen,
		ReadTimeout: readTimeout,
	}

	ab := common.NewAndreiboard(bufSize, opener, "")
	return &DUTControlAndreiboard{client: dutControlClient, Andreiboard: ab}
}

// FlashImage flashes image at the specified path on localhost to the board.
func (a *DUTControlAndreiboard) FlashImage(ctx context.Context, image string) (err error) {
	imageBytes, err := ioutil.ReadFile(image)
	if err != nil {
		return errors.Wrapf(err, "reading image file %q", image)
	}

	// Close and Re-open the port because opentitantool console occupies the UART that rescue uses.
	wasOpen := a.IsOpen()
	err = a.Close(ctx)
	if err != nil {
		return errors.Wrap(err, "close console before rescue")
	}
	defer func() {
		if wasOpen {
			if e := a.Open(ctx); e != nil && err == nil {
				err = e
			}
		}
	}()

	var args []*dutcontrol.CommandArg
	args = append(args, &dutcontrol.CommandArg{Type: &dutcontrol.CommandArg_Plain{Plain: "-p"}})
	args = append(args, &dutcontrol.CommandArg{Type: &dutcontrol.CommandArg_Plain{Plain: "Rescue"}})
	args = append(args, &dutcontrol.CommandArg{Type: &dutcontrol.CommandArg_File{File: imageBytes}})
	req := &dutcontrol.CommandRequest{Command: "bootstrap", Args: args}

	resp, err := a.client.Command(ctx, req)
	if err != nil {
		return errors.Wrap(err, "bootstrap request")
	}
	if resp.Err != "" {
		return errors.Errorf("bootstrap operation failed: %s", resp.Err)
	}
	return nil
}

// PlainCommand executes a opentitantool subcommand that uses no file arguments.
func (a *DUTControlAndreiboard) PlainCommand(ctx context.Context, cmd string, args ...string) (output []byte, err error) {
	var cArgs []*dutcontrol.CommandArg
	for _, a := range args {
		cArgs = append(cArgs, &dutcontrol.CommandArg{Type: &dutcontrol.CommandArg_Plain{Plain: a}})
	}
	req := &dutcontrol.CommandRequest{Command: cmd, Args: cArgs}
	resp, err := a.client.Command(ctx, req)
	if err != nil {
		return nil, errors.Wrapf(err, "request %s %s", cmd, strings.Join(args, " "))
	}
	if resp.Err != "" {
		return resp.Output, errors.Errorf("operation %s %s: %s", cmd, strings.Join(args, " "), resp.Err)
	}
	return resp.Output, nil
}

// OpenTitanToolCommand runs an arbitrary OpenTitan tool command (without up-/downloading any files).
func (a *DUTControlAndreiboard) OpenTitanToolCommand(ctx context.Context, cmd string, args ...string) (output []byte, err error) {
	return a.PlainCommand(ctx, cmd, args...)
}

// Reset resets the board via spiflash, causing the image to reboot.
func (a *DUTControlAndreiboard) Reset(ctx context.Context) error {
	_, err := a.PlainCommand(ctx, "gpio", "write", "RESET", "false")
	if err != nil {
		return err
	}

	_, err = a.PlainCommand(ctx, "gpio", "write", "RESET", "true")
	if err != nil {
		return err
	}
	return nil
}

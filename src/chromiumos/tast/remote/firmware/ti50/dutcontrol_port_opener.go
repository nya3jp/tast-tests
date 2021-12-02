// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ti50

import (
	"context"
	"io"
	"time"

	"chromiumos/tast/common/firmware/serial"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/ti50/dutcontrol"
	"chromiumos/tast/testing"
)

const (
	// ConsoleUart is the rawuart port.
	ConsoleUart = "console"
	// ConsoleBaud is the baud rate to use to access the ConsoleUart.
	ConsoleBaud = 115200

	// qSize is the channel size for the data and write receive channels, it should be large enough to not ever block on writes so that one channel does not block the other.
	qSize = 10000
)

// DUTControlRawUARTPortOpener opens a raw UART port through the dutcontrol grpc client.
//
// Example:
// conn, err := grpc.DialContext(ctx, hostPort, grpc.WithInsecure())
// if err != nil {
//     return nil, err
// }
// defer conn.Close(ctx)
// client := dutcontrol.NewDutControlClient(conn)
// opener := &DUTControlRawUARTPortOpener(client, "console", 115200, 1024, 200 * time.Millisecond)
// port, err := opener.OpenPort(ctx)
type DUTControlRawUARTPortOpener struct {
	// The dutcontrol grpc client.
	Client dutcontrol.DutControlClient
	// The uart number or an alias defined in the OpenTitanTool conf json file.
	Uart string
	// The baud rate.
	Baud int
	// The max size of received serial data.
	DataLen int
	// The timeout for a read operation.
	ReadTimeout time.Duration
}

// openDUTControlConsole opens a console and returns its data and write result receive channels.
func openDUTControlConsole(stream dutcontrol.DutControl_ConsoleClient, req *dutcontrol.ConsoleRequest) (<-chan *dutcontrol.ConsoleSerialData, <-chan *dutcontrol.ConsoleSerialWriteResult, error) {
	if err := stream.Send(req); err != nil {
		return nil, nil, errors.Wrap(err, "send request")
	}
	resp, err := stream.Recv()
	if err != nil {
		return nil, nil, errors.Wrap(err, "recv open")
	}
	open := resp.GetOpen()
	if open == nil {
		return nil, nil, errors.New("open response is nil")
	}
	if open.Err != "" {
		return nil, nil, errors.New(string(open.Err))
	}
	data := make(chan *dutcontrol.ConsoleSerialData, qSize)
	write := make(chan *dutcontrol.ConsoleSerialWriteResult, qSize)
	go func() {
	Loop:
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				testing.ContextLog(stream.Context(), "Dutcontrol recv EOF")
				break
			} else if err != nil {
				break
			}
			switch op := resp.Type.(type) {
			case *dutcontrol.ConsoleResponse_SerialData:
				data <- op.SerialData
			case *dutcontrol.ConsoleResponse_SerialWrite:
				write <- op.SerialWrite
			default:
				testing.ContextLog(stream.Context(), "Dutcontrol recv error, unknown message type: ", op)
				break Loop
			}
		}
		close(data)
		close(write)
	}()
	return data, write, nil
}

// OpenPort opens and returns the port.
func (c *DUTControlRawUARTPortOpener) OpenPort(ctx context.Context) (serial.Port, error) {
	stream, err := c.Client.Console(ctx)
	if err != nil {
		return nil, err
	}

	data, write, err := openDUTControlConsole(stream,
		&dutcontrol.ConsoleRequest{
			Operation: &dutcontrol.ConsoleRequest_Open{
				Open: &dutcontrol.ConsoleOpen{
					Type: &dutcontrol.ConsoleOpen_RawUart{RawUart: &dutcontrol.ConsoleOpenRawUART{Uart: c.Uart, Baud: int32(c.Baud), DataLen: int32(c.DataLen)}},
				},
			}})
	if err != nil {
		return nil, err
	}

	return &DUTControlPort{stream, data, write, c.ReadTimeout, nil}, nil
}

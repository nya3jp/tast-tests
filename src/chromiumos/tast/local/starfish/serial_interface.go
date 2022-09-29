// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package starfish

import (
	"context"
	"time"

	fserial "chromiumos/tast/common/firmware/serial"
	"chromiumos/tast/errors"
)

// SerialInterface holds data pertaining to the serial interface exposed on Starfish module
type SerialInterface struct {
	port fserial.Port
}

// Open handles the open of the Starfish serial interface.
func (s *SerialInterface) Open(ctx context.Context, devName string, baud int, readTimeout time.Duration) error {
	p, err := fserial.NewConnectedPortOpener(devName, baud, readTimeout).OpenPort(ctx)
	if err != nil {
		s.port = nil
		return errors.Wrap(err, "open failed")
	}
	s.port = p
	return nil
}

// Read handles raw read from the Starfish serial interface.
func (s *SerialInterface) Read(ctx context.Context, buf []byte) (int, error) {
	if s.port == nil {
		return 0, errors.New("Read: port not open")
	}
	n, err := s.port.Read(ctx, buf)
	return n, err
}

// Write handles raw write to the Starfish serial interface.
func (s *SerialInterface) Write(ctx context.Context, buf []byte) (int, error) {
	if s.port == nil {
		return 0, errors.New("Write: port not open")
	}
	n, err := s.port.Write(ctx, buf)
	return n, err
}

// Flush handles the flush operation on the Starfish output buffer.
func (s *SerialInterface) Flush(ctx context.Context) error {
	if s.port == nil {
		return errors.New("Flush: port not open")
	}
	return s.port.Flush(ctx)

}

// Close handles the close of the Starfish serial interface.
func (s *SerialInterface) Close(ctx context.Context) error {
	if s.port == nil {
		return errors.New("Close: port not open")
	}
	err := s.port.Close(ctx)
	s.port = nil
	return err
}

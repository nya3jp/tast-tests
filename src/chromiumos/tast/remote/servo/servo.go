// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package servo is used to communicate with servo devices connected to DUTs.
// It communicates with servod over XML-RPC.
// More details on servo: https://www.chromium.org/chromium-os/servo
package servo

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/divan/gorilla-xmlrpc/xml"

	"chromiumos/tast/errors"
)

// EchoRequest is the format of the arguments for the echo method.
type EchoRequest struct {
	// Message is the first string argument.
	Message string
}

// EchoReply is the return format of the echo method.
type EchoReply struct {
	// Message is the first string return value.
	Message string
}

// PowerNormalPressRequest is the format of the arguments for the
// power_normal_press method.
type PowerNormalPressRequest struct{}

// PowerNormalPressReply is the return format of the power_normal_press method.
type PowerNormalPressReply struct {
	// Message is the return value.
	Message bool
}

// Servo holds the connection information to a servod XML-RPC server for
// controlling a Servo.
type Servo struct {
	Host string
	Port int
}

const (
	// servodDefaultHost is the default host for servod.
	servodDefaultHost = "localhost"
	// servodDefaultPort is the default port for servod.
	servodDefaultPort = 9999
	// rpcTimeout is the default and maximum timeout for XML-RPC requests to servod.
	rpcTimeout = 10 * time.Second
)

// New initializes and returns a new Servo struct.
func New(ctx context.Context, host string, port int) (*Servo, error) {
	s := &Servo{host, port}

	// Ensure Servo is set up properly before returning.
	return s, s.verifyConnectivity(ctx)
}

// Default returns a new Servo struct with default values.
func Default(ctx context.Context) (*Servo, error) {
	return New(ctx, servodDefaultHost, servodDefaultPort)
}

func (s *Servo) verifyConnectivity(ctx context.Context) error {
	reply, err := s.Echo(ctx, EchoRequest{"hello from servo"})
	if err != nil {
		return err
	}

	const expectedMessage = "ECH0ING: hello from servo"
	if reply.Message != expectedMessage {
		return errors.Errorf("echo verification request returned %q; expected %q", reply.Message, expectedMessage)
	}

	return nil
}

// Echo calls the Servo echo method.
func (s *Servo) Echo(ctx context.Context, req EchoRequest) (EchoReply, error) {
	reply := EchoReply{}

	buf, err := xml.EncodeClientRequest("echo", &req)
	if err != nil {
		return reply, err
	}
	replyBytes, err := s.call(ctx, buf)
	if err != nil {
		return reply, err
	}

	err = xml.DecodeClientResponse(bytes.NewBuffer(replyBytes), &reply)
	return reply, err
}

// PowerNormalPress calls the Servo power_normal_press method.
func (s *Servo) PowerNormalPress(ctx context.Context, req PowerNormalPressRequest) (PowerNormalPressReply, error) {
	reply := PowerNormalPressReply{}

	buf, err := xml.EncodeClientRequest("power_normal_press", &req)
	if err != nil {
		return reply, err
	}
	replyBytes, err := s.call(ctx, buf)
	if err != nil {
		return reply, err
	}

	err = xml.DecodeClientResponse(bytes.NewBuffer(replyBytes), &reply)
	return reply, err
}

// call makes an XML-RPC call to servod.
func (s *Servo) call(ctx context.Context, body []byte) ([]byte, error) {
	// Get RPC timeout duration from context or use default.
	timeout := rpcTimeout
	if dl, ok := ctx.Deadline(); ok {
		newTimeout := dl.Sub(time.Now())
		// Timeout is min(deadline - now, rpcTimeout).
		if newTimeout < rpcTimeout {
			timeout = newTimeout
		}
	}

	servodURL := fmt.Sprintf("http://%s:%d", s.Host, s.Port)
	httpClient := &http.Client{Timeout: timeout}

	resp, err := httpClient.Post(servodURL, "text/xml", bytes.NewBuffer(body))
	if err != nil {
		return []byte{}, err
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}

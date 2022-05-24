// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testlibs allows Tast tests to connect to the Test Libs Server.
package testlibs

import (
	"context"

	pb "go.chromium.org/chromiumos/config/go/test/api"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
)

// LibsService represents connection to the Test Libs Service.
type LibsService struct {
	client pb.TestLibsServiceClient
	conn   *grpc.ClientConn
	libs   map[string]*TestLib
}

// NewLibsService connects to the TestLibsService and returns a struct to represent that connection.
func NewLibsService() (*LibsService, error) {
	// TODO kathrelkeld@: get port from test info rather than a hardcoded port
	addr := "localhost:1108"
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	client := pb.NewTestLibsServiceClient(conn)
	c := &LibsService{
		client: client,
		conn:   conn,
		libs:   make(map[string]*TestLib),
	}
	return c, nil
}

// Close requests to kill all running containers and stops the connection to TestLibsService.
func (ls *LibsService) Close(ctx context.Context) {
	for libID := range ls.libs {
		ls.libs[libID].Close(ctx)
	}
	ls.conn.Close()
}

// StartLib requests that the TestLibsService start up a docker container with the given name.
func (ls *LibsService) StartLib(ctx context.Context, name string, options ...string) (*TestLib, error) {
	startReq := &pb.GetLibRequest{
		Name:    name,
		Version: "0",
	}
	resp, err := ls.client.StartLib(ctx, startReq)
	if err != nil {
		return nil, err
	}
	if fail := resp.GetFailure(); fail != nil {
		return nil, errors.New(pb.GetLibFailure_Reason_name[int32(fail.Reason)])
	}
	l := &TestLib{
		name:    name,
		id:      resp.GetSuccess().Id,
		service: ls,
		running: true,
	}
	ls.libs[l.id] = l

	return l, nil
}

// TestLib represents a single running docker container.
type TestLib struct {
	name    string
	id      string
	service *LibsService
	running bool
}

// Close requests that the TestLibsService kill the docker container.
func (l *TestLib) Close(ctx context.Context) {
	if !l.running {
		return
	}
	req := &pb.KillLibRequest{
		Id: l.id,
	}
	_, _ = l.service.client.KillLib(ctx, req)
	l.running = false
}

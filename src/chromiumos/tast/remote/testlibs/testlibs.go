// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testlibs allows Tast tests to connect to the Test Libs Server.
package testlibs

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"google.golang.org/grpc"

	//TODO update this when it's checked in
	pb "chromiumos/tast/services/cros/example/testlibs"
)

// LibsService represents connection to the Test Libs Service.
type LibsService struct {
	client pb.TestLibsServiceClient
	conn   *grpc.ClientConn
	libs   map[string]*TestLib
}

// NewLibsService connects to the TestLibsService and returns a struct to represent that connection.
func NewLibsService() (*LibsService, error) {
	// TODO: get port from test info rather than a hardcoded port
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
// User can use this struct to call commands on this test library.
type TestLib struct {
	name    string
	id      string
	service *LibsService
	running bool
}

// RunCmd sends a run request to the docker container library via TestLibsService.
func (l *TestLib) RunCmd(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	if !l.running {
		return nil, errors.New("cannot run a command on a closed library")
	}
	runReq := &pb.RunLibCmdRequest{
		Id:   l.id,
		Cmd:  cmd,
		Args: strings.Join(args, ","),
	}
	resp, err := l.service.client.RunCmd(ctx, runReq)
	if err != nil {
		return nil, err
	}
	if out := resp.GetFailure(); out != nil {
		return nil, errors.New(pb.RunLibCmdFailure_Reason_name[int32(out.Reason)])
	}
	return resp.Output, err
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

// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/common"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterConnServiceServer(srv,
				&ConnService{sharedObject: common.SharedObjectsForServiceSingleton, conns: make(map[string]*chrome.Conn), idGenerator: incrementer()})
		},
		GuaranteeCompatibility: true,
	})
}

// ConnService implements tast.cros.ui.ConnService.
type ConnService struct {
	sharedObject *common.SharedObjectsForService
	conns        map[string]*chrome.Conn
	idGenerator  func() string
}

func incrementer() func() string {
	// TODO: Probably just enough to have nextId field.
	i := 0
	return func() string {
		i++
		return strconv.Itoa(i)
	}
}

func (svc *ConnService) Open(ctx context.Context, req *pb.OpenRequest) (*structpb.Value, error) {
	conn, err := svc.sharedObject.Chrome.NewConn(ctx, req.Url)
	if err != nil {
		return nil, err
	}

	id := svc.idGenerator()
	svc.conns[id] = conn

	return structpb.NewValue(id)
}

func (svc *ConnService) Close(ctx context.Context, req *pb.CloseRequest) (*empty.Empty, error) {
	id := req.Id
	conn, err := svc.getConnByID(id)
	if err != nil {
		return &empty.Empty{}, err
	}

	if err := conn.Close(); err != nil {
		delete(svc.conns, id)
		return &empty.Empty{}, err
	}

	delete(svc.conns, id)

	return &empty.Empty{}, nil
}

func (svc *ConnService) Eval(ctx context.Context, req *pb.ConnEvalRequest) (*structpb.Value, error) {
	conn, err := svc.getConnByID(req.Id)
	if err != nil {
		return nil, err
	}

	var out interface{}
	if err := conn.Eval(ctx, req.Expr, &out); err != nil {
		if err == chrome.ErrTestConnUndefinedOut {
			return &structpb.Value{}, nil
		}
		return nil, errors.Wrap(err, "conn.Eval() failed")
	}

	return structpb.NewValue(out)
}

func (svc *ConnService) Call(ctx context.Context, req *pb.ConnCallRequest) (*structpb.Value, error) {
	conn, err := svc.getConnByID(req.Id)
	if err != nil {
		return nil, err
	}

	var out interface{}
	var args []interface{}
	for _, arg := range req.Args {
		args = append(args, arg.AsInterface())
	}
	if err := conn.Call(ctx, &out, req.Fn, args...); err != nil {
		if err == chrome.ErrTestConnUndefinedOut {
			return &structpb.Value{}, nil
		}
		return nil, err
	}
	return structpb.NewValue(out)
}

func (svc *ConnService) WaitForExpr(ctx context.Context, req *pb.ConnWaitForExprRequest) (*empty.Empty, error) {
	conn, err := svc.getConnByID(req.Id)
	if err != nil {
		return &empty.Empty{}, err
	}

	if req.FailOnErr {
		return &empty.Empty{}, conn.WaitForExprFailOnErrWithTimeout(ctx, req.Expr, time.Second*time.Second*time.Duration(req.TimeoutSecs))
	}
	return &empty.Empty{}, conn.WaitForExprWithTimeout(ctx, req.Expr, time.Second*time.Duration(req.TimeoutSecs))
}

func (svc *ConnService) getConnByID(id string) (*chrome.Conn, error) {
	if id == "" {
		return nil, errors.New("no id provided")
	}

	conn, ok := svc.conns[id]
	if !ok {
		return nil, errors.Errorf("no conn for id = %s", id)
	}

	return conn, nil
}

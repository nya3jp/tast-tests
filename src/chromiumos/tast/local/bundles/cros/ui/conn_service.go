// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"sync"
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
				&ConnService{sharedObject: common.SharedObjectsForServiceSingleton, conns: make(map[uint32]*chrome.Conn), idGenerator: incrementer()})
		},
		GuaranteeCompatibility: true,
	})
}

// ConnService implements tast.cros.ui.ConnService.
// TODO(crbug.com/1378851): Implement a way to automatically release stale conns.
type ConnService struct {
	sharedObject *common.SharedObjectsForService
	conns        map[uint32]*chrome.Conn
	idGenerator  func() uint32
	mutex        sync.Mutex
}

func incrementer() func() uint32 {
	var i uint32 = 0
	return func() uint32 {
		i++
		return i
	}
}

// NewConn opens a new tab with the provided url and creates a new Conn for it.
// Conns created should be deleted with a call to Close|CloseAll.
func (svc *ConnService) NewConn(ctx context.Context, req *pb.NewConnRequest) (*pb.NewConnResponse, error) {
	conn, err := svc.sharedObject.Chrome.NewConn(ctx, req.Url)
	if err != nil {
		return nil, err
	}

	svc.mutex.Lock()
	defer svc.mutex.Unlock()
	id := svc.idGenerator()
	svc.conns[id] = conn

	return &pb.NewConnResponse{Id: id}, nil
}

// NewConnForTarget creates a new Conn for an existing tab matching the url provided.
// Conns created should be deleted with a call to Close|CloseAll.
func (svc *ConnService) NewConnForTarget(ctx context.Context, req *pb.NewConnForTargetRequest) (*pb.NewConnResponse, error) {
	conn, err := svc.sharedObject.Chrome.NewConnForTarget(ctx, chrome.MatchTargetURL(req.Url))
	if err != nil {
		return nil, err
	}

	svc.mutex.Lock()
	defer svc.mutex.Unlock()
	id := svc.idGenerator()
	svc.conns[id] = conn

	return &pb.NewConnResponse{Id: id}, nil
}

// ActivateTarget calls conn.ActivateTarget.
func (svc *ConnService) ActivateTarget(ctx context.Context, req *pb.ActivateTargetRequest) (*empty.Empty, error) {
	conn, err := svc.connByID(req.Id)
	if err != nil {
		return nil, err
	}

	if err := conn.ActivateTarget(ctx); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}

// Navigate calls conn.Navigate.
func (svc *ConnService) Navigate(ctx context.Context, req *pb.NavigateRequest) (*empty.Empty, error) {
	conn, err := svc.connByID(req.Id)
	if err != nil {
		return nil, err
	}

	if err := conn.Navigate(ctx, req.Url); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}

// Close calls conn.Close.
func (svc *ConnService) Close(ctx context.Context, req *pb.CloseRequest) (*empty.Empty, error) {
	id := req.Id
	conn, err := svc.connByID(id)
	if err != nil {
		return nil, err
	}

	delete(svc.conns, id)
	if err := conn.Close(); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}

// CloseAll closes all conns.
func (svc *ConnService) CloseAll(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	var errs []error
	for id, conn := range svc.conns {
		delete(svc.conns, id)
		if err := conn.Close(); err != nil {
			errs = append(errs, errors.Wrapf(err, "conn.Close() failed for id = %d", id))
		}
	}

	if errs != nil {
		if len(errs) == 1 {
			return nil, errs[0]
		}

		return nil, errors.Wrapf(errs[0], " (and %d other error(s)", len(errs)-1)
	}

	return &empty.Empty{}, nil
}

// Eval calls conn.Eval.
func (svc *ConnService) Eval(ctx context.Context, req *pb.ConnEvalRequest) (*structpb.Value, error) {
	conn, err := svc.connByID(req.Id)
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

// Call calls conn.Call.
func (svc *ConnService) Call(ctx context.Context, req *pb.ConnCallRequest) (*structpb.Value, error) {
	conn, err := svc.connByID(req.Id)
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

// WaitForExpr calls conn.WaitForExpr.
func (svc *ConnService) WaitForExpr(ctx context.Context, req *pb.ConnWaitForExprRequest) (*empty.Empty, error) {
	conn, err := svc.connByID(req.Id)
	if err != nil {
		return nil, err
	}

	if req.FailOnErr {
		if err := conn.WaitForExprFailOnErrWithTimeout(ctx, req.Expr, time.Second*time.Second*time.Duration(req.TimeoutSecs)); err != nil {
			return nil, err
		}
		return &empty.Empty{}, nil
	}
	return &empty.Empty{}, conn.WaitForExprWithTimeout(ctx, req.Expr, time.Second*time.Duration(req.TimeoutSecs))
}

func (svc *ConnService) connByID(id uint32) (*chrome.Conn, error) {
	conn, ok := svc.conns[id]
	if !ok {
		return nil, errors.Errorf("no conn exists for id = %d", id)
	}

	return conn, nil
}

// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/common"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterTconnServiceServer(srv,
				&TconnService{sharedObject: common.SharedObjectsForServiceSingleton})
		},
		GuaranteeCompatibility: true,
	})
}

// TconnService implements tast.cros.ui.TconnService.
type TconnService struct {
	sharedObject *common.SharedObjectsForService
}

// Eval calls tconn.Eval.
func (svc *TconnService) Eval(ctx context.Context, req *pb.EvalRequest) (*structpb.Value, error) {
	return common.UseTconn(ctx, svc.sharedObject, func(tconn *chrome.TestConn) (*structpb.Value, error) {
		var out interface{}
		if err := tconn.Eval(ctx, req.Expr, &out); err != nil {
			if err == chrome.ErrTestConnUndefinedOut {
				return &structpb.Value{}, nil
			}
			return nil, err
		}
		return structpb.NewValue(out)
	})
}

// Call calls tconn.Call.
func (svc *TconnService) Call(ctx context.Context, req *pb.CallRequest) (*structpb.Value, error) {
	return common.UseTconn(ctx, svc.sharedObject, func(tconn *chrome.TestConn) (*structpb.Value, error) {
		var out interface{}
		var args []interface{}
		for _, arg := range req.Args {
			args = append(args, arg.AsInterface())
		}
		if err := tconn.Call(ctx, &out, req.Fn, args...); err != nil {
			if err == chrome.ErrTestConnUndefinedOut {
				return &structpb.Value{}, nil
			}
			return nil, err
		}
		return structpb.NewValue(out)
	})
}

// WaitForExpr calls tconn.Eval.
func (svc *TconnService) WaitForExpr(ctx context.Context, req *pb.WaitForExprRequest) (*empty.Empty, error) {
	return common.UseTconn(ctx, svc.sharedObject, func(tconn *chrome.TestConn) (*empty.Empty, error) {
		if req.FailOnErr {
			return &empty.Empty{}, tconn.WaitForExprFailOnErrWithTimeout(ctx, req.Expr, time.Second*time.Duration(req.TimeoutSecs))
		}
		return &empty.Empty{}, tconn.WaitForExprWithTimeout(ctx, req.Expr, time.Second*time.Duration(req.TimeoutSecs))
	})
}

// ResetAutomation calls tconn.ResetAutomation.
func (svc *TconnService) ResetAutomation(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	return common.UseTconn(ctx, svc.sharedObject, func(tconn *chrome.TestConn) (*empty.Empty, error) {
		return &empty.Empty{}, tconn.ResetAutomation(ctx)
	})
}

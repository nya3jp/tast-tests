// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"

	"chromiumos/tast/remote/crosserverutil"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TconnServiceGRPC,
		Desc:         "Check basic functionalities of UI TconnService",
		Contacts:     []string{"msta@google.com", "chromeos-engprod-syd@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"grpcServerPort"},
		HardwareDeps: hwdep.D(hwdep.Model("betty")),
		LacrosStatus: testing.LacrosVariantUnneeded,
		Timeout:      time.Minute * 5,
	})
}

// TconnServiceGRPC tests basic functionalities of UI TconnService.
func TconnServiceGRPC(ctx context.Context, s *testing.State) {
	grpcServerPort := crosserverutil.DefaultGRPCServerPort
	if portStr, ok := s.Var("grpcServerPort"); ok {
		if portInt, err := strconv.Atoi(portStr); err == nil {
			grpcServerPort = portInt
		}
	}

	// Connect to TCP based gRPC Server on DUT.
	cl, err := crosserverutil.Dial(ctx, s.DUT(), "localhost", grpcServerPort, true)
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Start Chrome on the DUT.
	cs := pb.NewChromeServiceClient(cl.Conn)
	loginReq := &pb.NewRequest{}
	if _, err := cs.New(ctx, loginReq, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cs.Close(ctx, &empty.Empty{})

	svc := pb.NewTconnServiceClient(cl.Conn)

	value := func(val interface{}) *structpb.Value {
		result, err := structpb.NewValue(val)
		if err != nil {
			s.Fatal("Invalid value: ", err)
		}
		return result
	}

	structValue := value(map[string]interface{}{"a": 1, "b": 2})
	for _, tc := range []struct {
		expr string
		want *structpb.Value
	}{
		{
			expr: "1 + 1",
			want: value(2),
		},
		{
			expr: "tast.promisify(chrome.i18n.getAcceptLanguages)()",
			want: value([]interface{}{"en-US", "en"}),
		},
		{
			expr: "(() => { return {a: 1, b: 2} })()",
			want: structValue,
		},
	} {
		got, err := svc.Eval(ctx, &pb.EvalRequest{Expr: tc.expr})

		if err != nil {
			s.Fatalf("Eval(%s) failed: %v", tc.expr, err)
		}
		if diff := cmp.Diff(got, tc.want, protocmp.Transform()); diff != "" {
			s.Fatalf("Eval(%s) mismatch (-got +want):%s", tc.expr, diff)
		}
	}

	for _, tc := range []struct {
		fn   string
		args []*structpb.Value
		want *structpb.Value
	}{
		{
			fn:   "(x, y) => { return x + y; }",
			args: []*structpb.Value{value(1), value(2)},
			want: value(3),
		},
		{
			fn:   "(x) => { return x; }",
			args: []*structpb.Value{structValue},
			want: structValue,
		},
	} {
		got, err := svc.Call(ctx, &pb.CallRequest{Fn: tc.fn, Args: tc.args})

		if err != nil {
			s.Fatalf("Call %s with args %v failed: %v", tc.fn, tc.args, err)
		}
		if diff := cmp.Diff(got, tc.want, protocmp.Transform()); diff != "" {
			s.Fatalf("Call %s with args %v mismatch (-got +want):%s", tc.fn, tc.args, diff)
		}
	}

}

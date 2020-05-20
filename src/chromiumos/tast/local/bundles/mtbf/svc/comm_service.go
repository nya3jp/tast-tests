// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package svc

import (
	"encoding/base64"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/services/mtbf/svc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			svc.RegisterCommServiceServer(srv, &CommService{chrome.SvcLoginReusePre{S: s}})
		},
	})
}

// CommService implements tast.mtbf.svc.CommService.
type CommService struct {
	//Use embedded structure to get the implementation of LoginReusePre
	chrome.SvcLoginReusePre
}

// Login Login
func (c *CommService) Login(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {

	testing.ContextLog(ctx, "CommService - Login called")

	err := c.PrePrepare(ctx) // prepare the Chrome instance just in case
	if err != nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}
	return &empty.Empty{}, nil

}

// TakeScreenshot TakeScreenshot
func (c *CommService) TakeScreenshot(ctx context.Context, req *empty.Empty) (*svc.Screenshot, error) {

	testing.ContextLog(ctx, "CommService - TakeScreenshot called")

	err := c.PrePrepare(ctx) // prepare the Chrome instance just in case
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}

	conn, err := c.CR.TestAPIConn(ctx)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeTestConn, err)
	}

	var base64PNG string
	script :=
		`new Promise(function(resolve, reject) {
			chrome.autotestPrivate.takeScreenshot(function(base64PNG) {
				if (chrome.runtime.lastError === undefined) {
					resolve(base64PNG);
				} else {
					reject(chrome.runtime.lastError.message);
				}
			});
		})`

	if err := conn.EvalPromise(ctx, script, &base64PNG); err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeExeJs, err, "TakeScreenshot")
	}

	sDec, err := base64.StdEncoding.DecodeString(base64PNG)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.GRPCBase64Decode, err)
	}

	return &svc.Screenshot{Content: sDec}, nil

}

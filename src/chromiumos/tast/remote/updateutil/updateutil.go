// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updateutil

import (
	"context"
	"encoding/json"
	"math/rand"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/empty"
	"go.chromium.org/chromiumos/config/go/api/test/tls"
	"go.chromium.org/chromiumos/config/go/api/test/tls/dependencies/longrunning"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/rpc"
	aupb "chromiumos/tast/services/cros/autoupdate"
	"chromiumos/tast/testing"
)

// ImageVersion gets the DUT image version from the parsed /etc/lsb-realse file.
func ImageVersion(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint) (string, error) {
	return lsbReleaseEntry(ctx, dut, rpcHint, lsbrelease.Version)
}

// ImageBuilderPath gets the DUT image builder path from the parsed /etc/lsb-realse file.
func ImageBuilderPath(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint) (string, error) {
	return lsbReleaseEntry(ctx, dut, rpcHint, lsbrelease.BuilderPath)
}

func lsbReleaseEntry(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint, key string) (string, error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	// Connect to DUT.
	cl, err := rpc.Dial(ctx, dut, rpcHint, "cros")
	if err != nil {
		return "", errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(cleanupCtx)

	// Check the new image version.
	updateClient := aupb.NewUpdateServiceClient(cl.Conn)

	response, err := updateClient.LSBReleaseContent(ctx, &empty.Empty{})
	if err != nil {
		return "", errors.Wrap(err, "failed to read lsb-release")
	}

	var lsb map[string]string
	if err := json.Unmarshal(response.ContentJson, &lsb); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal lsb-relese content")
	}

	entry, ok := lsb[key]
	if !ok {
		return "", errors.Wrapf(err, "failed to get entry value for key %q from lsb-release content", key)
	}

	return entry, nil
}

// CacheForDUT caches the requred update files in a caching server which is available from the DUT.
func CacheForDUT(ctx context.Context, dut *dut.DUT, TLWAddress, gsPathPrefix string) (string, error) {
	conn, err := grpc.Dial(TLWAddress, grpc.WithInsecure())
	if err != nil {
		return "", err
	}
	defer conn.Close()

	c := tls.NewWiringClient(conn)

	op, err := c.CacheForDut(ctx, &tls.CacheForDutRequest{
		Url:     gsPathPrefix,
		DutName: dut.HostName(),
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to start CacheForDut request")
	}

	op, err = lroWait(ctx, longrunning.NewOperationsClient(conn), op.Name)
	if err != nil {
		return "", errors.Wrap(err, "failed to wait for CacheForDut")
	}

	if s := op.GetError(); s != nil {
		return "", errors.Errorf("failed to get CacheForDut, %s", s)
	}

	a := op.GetResponse()
	if a == nil {
		return "", errors.Errorf("failed to get CacheForDut response for URL=%s and Name=%s", gsPathPrefix, dut.HostName())
	}

	resp := &tls.CacheForDutResponse{}
	if err := ptypes.UnmarshalAny(a, resp); err != nil {
		return "", errors.Errorf("unexpected response from CacheForDut, %v", a)
	}

	testing.ContextLogf(ctx, "The cache URL for %q is %q", gsPathPrefix, resp.GetUrl())
	return resp.GetUrl(), nil
}

// lroWait is duplicate of the function from an infra package, which is not available in Tast.
// It uses different types than the one at src/platform/dev/src/chromiumos/lro/lroWait.go.
func lroWait(ctx context.Context, client longrunning.OperationsClient, name string, opts ...grpc.CallOption) (*longrunning.Operation, error) {
	// Exponential backoff is used for retryable gRPC errors. In future, we
	// may want to make these parameters configurable.
	const initialBackoffMillis = 1000
	const maxAttempts = 4
	attempt := 0

	// WaitOperation() can return before the provided timeout even though the
	// underlying operation is in progress. It may also fail for retryable
	// reasons. Thus, we must loop until timeout ourselves.
	for {
		// WaitOperation respects timeout in the RPC Context as well as through
		// an explicit field in WaitOperationRequest. We depend on Context
		// cancellation for timeouts (like everywhere else in this codebase).
		// On timeout, WaitOperation() will return an appropriate error
		// response.
		op, err := client.WaitOperation(ctx, &longrunning.WaitOperationRequest{
			Name: name,
		}, opts...)
		switch status.Code(err) {
		case codes.OK:
			attempt = 0
		case codes.Unavailable, codes.ResourceExhausted:
			// Retryable error; retry with exponential backoff.
			if attempt >= maxAttempts {
				return op, err
			}
			delay := rand.Int63n(initialBackoffMillis * (1 << attempt))
			testing.Sleep(ctx, time.Duration(delay)*time.Millisecond) // The sleep method was changed.
			attempt++
		default:
			// Non-retryable error
			return op, err
		}
		if op.Done {
			return op, nil
		}
	}
}

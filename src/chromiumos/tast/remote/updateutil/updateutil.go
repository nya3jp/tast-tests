// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package updateutil

import (
	"context"
	"encoding/json"
	"math/rand"
	"net"
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
	return EntryFromLSBRelease(ctx, dut, rpcHint, lsbrelease.Version)
}

// ImageBuilderPath gets the DUT image builder path from the parsed /etc/lsb-realse file.
func ImageBuilderPath(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint) (string, error) {
	return EntryFromLSBRelease(ctx, dut, rpcHint, lsbrelease.BuilderPath)
}

// EntryFromLSBRelease is a wrapper for FillFromLSBRelease to get a single entry
// from the /etc/lsb-realse file with a simpler call.
func EntryFromLSBRelease(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint, key string) (string, error) {
	lsbContent := map[string]string{
		key: "",
	}
	err := FillFromLSBRelease(ctx, dut, rpcHint, lsbContent)
	if err != nil {
		return "", err
	}

	return lsbContent[key], nil
}

// FillFromLSBRelease fills map[string]string it gets as input with values
// form the /etc/lsb-realse file based on matching keys.
func FillFromLSBRelease(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint, req map[string]string) error {
	if req == nil || len(req) == 0 {
		return errors.New("request map should contain at least one key")
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	// Connect to DUT.
	cl, err := rpc.Dial(ctx, dut, rpcHint)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(cleanupCtx)

	updateClient := aupb.NewUpdateServiceClient(cl.Conn)

	response, err := updateClient.LSBReleaseContent(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "failed to read lsb-release")
	}

	var lsb map[string]string
	if err := json.Unmarshal(response.ContentJson, &lsb); err != nil {
		return errors.Wrap(err, "failed to unmarshal lsb-relese content")
	}

	missingKeys := make([]string, 0, len(req))
	for key := range req {
		value, ok := lsb[key]
		if !ok {
			missingKeys = append(missingKeys, key)
			continue
		}
		req[key] = value
	}

	if len(missingKeys) > 0 {
		return errors.Errorf("the following keys were not found in lsb-release %#v", missingKeys)
	}

	return nil
}

// CacheForDUT caches the required update files in a caching server which is available from the DUT.
// The required files include the update payload and the payload metadata.
func CacheForDUT(ctx context.Context, dut *dut.DUT, TLWAddress, gsPathPrefix string) (string, error) {
	conn, err := grpc.Dial(TLWAddress, grpc.WithInsecure())
	if err != nil {
		return "", err
	}
	defer conn.Close()

	client := tls.NewWiringClient(conn)

	host, _, err := net.SplitHostPort(dut.HostName())
	if err != nil {
		return "", errors.Wrapf(err, "failed to extract DUT hostname from %q", dut.HostName())
	}

	operation, err := client.CacheForDut(ctx, &tls.CacheForDutRequest{
		Url:     gsPathPrefix,
		DutName: host,
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to start CacheForDut request")
	}

	// Wait until the long running operation of caching is completed.
	operation, err = lroWait(ctx, longrunning.NewOperationsClient(conn), operation.Name)
	if err != nil {
		return "", errors.Wrap(err, "failed to wait for CacheForDut")
	}

	if status := operation.GetError(); status != nil {
		return "", errors.Errorf("failed to get CacheForDut, %s", status)
	}

	respAny := operation.GetResponse()
	if respAny == nil {
		return "", errors.Errorf("failed to get CacheForDut response for URL=%s and Name=%s", gsPathPrefix, dut.HostName())
	}

	resp := &tls.CacheForDutResponse{}
	if err := ptypes.UnmarshalAny(respAny, resp); err != nil {
		return "", errors.Errorf("unexpected response from CacheForDut, %v", respAny)
	}

	testing.ContextLogf(ctx, "The cache URL for %q is %q", gsPathPrefix, resp.GetUrl())
	return resp.GetUrl(), nil
}

// lroWait waits until the long-running operation specified by the provided operation name is done.
// If the operation is already done, it returns immediately.
// Unlike OperationsClient's WaitOperation(), it only returns on context
// timeout or completion of the operation.
// lroWait is duplicate of the function from an infra package, which is not available in Tast.
// It uses different types than the one at src/platform/dev/src/chromiumos/lro/wait.go.
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

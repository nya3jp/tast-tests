// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bytes"
	"context"
	"encoding/hex"
	"math/rand"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/example"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
)

const testKeyNeverStored = "bootlockbox_unused"
const testKey = "bootlockbox_testkey"

func init() {
	testing.AddTest(&testing.Test{
		Func:         BootLockbox,
		Desc:         "Boot lockbox read/store test",
		Contacts:     []string{"xzhou@chromium.org", "victorhsieh@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"tpm2", "reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService", "tast.cros.example.ChromeService"},
	})
}

func login(ctx context.Context, s *testing.State, cl *rpc.Client) {
	cr := example.NewChromeServiceClient(cl.Conn)
	if _, err := cr.New(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx, &empty.Empty{})
}

func testReadNonExistingEntry(ctx context.Context, s *testing.State, client security.BootLockboxServiceClient) {
	s.Log("Test reading from non-existing entry")
	response, err := client.Read(ctx, &security.ReadBootLockboxRequest{Key: testKeyNeverStored})
	if err != nil {
		s.Fatal("Unexpected error when read from boot lockbox: ", err)
	}
	if len(response.Value) > 0 {
		s.Errorf("Reading from unused key should return empty result, but got %q (encoded)", hex.EncodeToString(response.Value))
	}
}

func testWriteReadConsistency(ctx context.Context, s *testing.State, client security.BootLockboxServiceClient) {
	s.Log("Test write then read consistency")
	expected := make([]byte, 8)
	rand.Read(expected)
	s.Logf("Random data: %s", hex.EncodeToString(expected))
	_, err := client.Store(ctx, &security.StoreBootLockboxRequest{Key: testKey, Value: expected})
	if err != nil {
		s.Error("Failed to store to boot lockbox: ", err)
	}

	response, err := client.Read(ctx, &security.ReadBootLockboxRequest{Key: testKey})
	if err != nil {
		s.Fatal("Failed to read from boot lockbox: ", err)
	}

	s.Logf("Key = %v, Value = %v", testKey, hex.EncodeToString(response.Value))
	s.Log("Length of Response = ", len(response.Value))

	response, err = client.Read(ctx, &security.ReadBootLockboxRequest{Key: testKey})
	if err != nil {
		s.Fatal("Unexpected error when read from boot lockbox: ", err)
	}
	if bytes.Compare(response.Value, expected) != 0 {
		s.Errorf("Retrieved value is inconsistent. Stored %q, retrieved %q", hex.EncodeToString(expected), hex.EncodeToString(response.Value))
	}
}

func testWriteFailureAfterLogin(ctx context.Context, s *testing.State, client security.BootLockboxServiceClient) {
	s.Log("Test write should fail after login")
	_, err := client.Store(ctx, &security.StoreBootLockboxRequest{Key: testKey, Value: []byte("123")})
	if err == nil {
		s.Error("Store should have failed but succeeded")
	} else {
		s.Log("Expected failure: ", err)
	}
}

func testRestoreTestKeyValue(ctx context.Context, s *testing.State, client security.BootLockboxServiceClient) {

	s.Log("Trying to restore the Test Key value to NULL")

	_, err := client.Store(ctx, &security.StoreBootLockboxRequest{Key: testKey, Value: []byte("0")})
	if err != nil {
		s.Error("Failed to store to boot lockbox: ", err)
	}

	response, err := client.Read(ctx, &security.ReadBootLockboxRequest{Key: testKey})
	if err != nil {
		s.Fatal("Failed to read from boot lockbox: ", err)
	}

	s.Logf("Key = %v, Expecting Null Value here = %v", testKey, hex.EncodeToString(response.Value))
	s.Log("Length of Response = ", len(response.Value))

	response, err = client.Read(ctx, &security.ReadBootLockboxRequest{Key: testKey})
	if err != nil {
		s.Fatal("Unexpected error when read from boot lockbox: ", err)
	}
	if bytes.Compare(response.Value, []byte("0")) != 0 {
		s.Errorf("Retrieved value is not Empty", hex.EncodeToString(response.Value))
	}
}

func BootLockbox(ctx context.Context, s *testing.State) {
	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Reboot to make boot lockbox writable
	s.Log("Rebooting")
	if err := s.DUT().Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}
	cl, err = rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Start actual tests
	client := security.NewBootLockboxServiceClient(cl.Conn)

	s.Logf("Starting test - marker")
	testReadNonExistingEntry(ctx, s, client)
	testWriteReadConsistency(ctx, s, client)

	login(ctx, s, cl) // this makes boot lockbox readonly
	testWriteFailureAfterLogin(ctx, s, client)

	// Reset the value to empty. Deletion is not currently supported.

	// Reboot to make boot lockbox writable
	s.Log("Rebooting Again")
	if err := s.DUT().Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}
	cl, err = rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Start actual tests
	client = security.NewBootLockboxServiceClient(cl.Conn)

	testRestoreTestKeyValue(ctx, s, client)
	//client.Store(ctx, &security.StoreBootLockboxRequest{Key: testKey, Value: n})
}

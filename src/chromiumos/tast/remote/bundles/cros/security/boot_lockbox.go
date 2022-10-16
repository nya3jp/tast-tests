// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bytes"
	"context"
	"encoding/hex"
	"math/rand"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
)

const testKeyNeverStored = "bootlockbox_unused"
const testKey = "bootlockbox_testkey"

func init() {
	testing.AddTest(&testing.Test{
		Func:         BootLockbox,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Boot lockbox read/store test",
		Contacts:     []string{"xzhou@chromium.org", "victorhsieh@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"tpm", "reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
	})
}

func login(ctx context.Context, client security.BootLockboxServiceClient) (*empty.Empty, error) {
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		return nil, errors.Wrap(err, "failed to start Chrome")
	}
	return client.CloseChrome(ctx, &empty.Empty{})
}

func testReadNonExistingEntry(ctx context.Context, client security.BootLockboxServiceClient) error {
	testing.ContextLog(ctx, "Test reading from non-existing entry")
	response, err := client.Read(ctx, &security.ReadBootLockboxRequest{Key: testKeyNeverStored})
	if err != nil {
		return errors.Wrap(err, "failed to read from boot lockbox")
	}
	if len(response.Value) > 0 {
		return errors.Errorf("Reading from unused key should return empty result, but got %q (encoded)", hex.EncodeToString(response.Value))
	}
	return nil
}

func testWriteReadConsistency(ctx context.Context, client security.BootLockboxServiceClient) error {
	testing.ContextLog(ctx, "Test write then read consistency")
	expected := make([]byte, 8)
	rand.Read(expected)
	testing.ContextLogf(ctx, "Random data: %s", hex.EncodeToString(expected))

	if _, err := client.Store(ctx, &security.StoreBootLockboxRequest{Key: testKey, Value: expected}); err != nil {
		return errors.Wrap(err, "failed to store key in boot lockbox")
	}

	response, err := client.Read(ctx, &security.ReadBootLockboxRequest{Key: testKey})
	if err != nil {
		return errors.Wrap(err, "failed to read from boot lockbox")
	}

	if !bytes.Equal(response.Value, expected) {
		return errors.Errorf("Retrieved value is inconsistent. Stored %q, retrieved %q", hex.EncodeToString(expected), hex.EncodeToString(response.Value))
	}
	return nil
}

func testWriteFailureAfterLogin(ctx context.Context, client security.BootLockboxServiceClient) error {
	testing.ContextLog(ctx, "Test write should fail after login")
	_, err := client.Store(ctx, &security.StoreBootLockboxRequest{Key: testKey, Value: []byte("123")})
	if err == nil {
		return errors.New("Boot lockbox store should have failed but succeeded")
	}
	testing.ContextLog(ctx, "Expected failure: ", err)
	return nil
}

func restoreTestKeyValue(ctx context.Context, client security.BootLockboxServiceClient) error {
	testing.ContextLog(ctx, "Trying to restore the testkey as 0 in boot lockbox")
	nullByte := []byte("0")
	_, err := client.Store(ctx, &security.StoreBootLockboxRequest{Key: testKey, Value: nullByte})
	if err != nil {
		testing.ContextLog(ctx, "Failed to store key in boot lockbox")
		return err
	}
	response, err := client.Read(ctx, &security.ReadBootLockboxRequest{Key: testKey})
	if err != nil {
		testing.ContextLog(ctx, "Failed to read from boot lockbox")
		return err
	}
	if !bytes.Equal(response.Value, nullByte) {
		return errors.Errorf("Retrieved value should be 0, but got %q", hex.EncodeToString(response.Value))
	}
	return nil
}

func rebootAndReconnect(ctx context.Context, s *testing.State) (*rpc.Client, error) {
	// Reboot to make boot lockbox writable
	testing.ContextLog(ctx, "Rebooting")
	if err := s.DUT().Reboot(ctx); err != nil {
		return nil, errors.Wrap(err, "unexpected error while rebooting DUT")
	}
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to RPC service on the DUT")
	}
	return cl, nil
}

func BootLockbox(ctx context.Context, s *testing.State) {
	r := hwsecremote.NewCmdRunner(s.DUT())
	helper, err := hwsecremote.NewHelper(r, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	// BootLockbox would only available when the TPM is ready.
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}

	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	if cl, err = rebootAndReconnect(ctx, s); err != nil {
		s.Fatal("Failed to reboot and reconnect to boot lockbox service")
	}
	defer cl.Close(ctx)

	// Start actual tests
	client := security.NewBootLockboxServiceClient(cl.Conn)

	if err := testReadNonExistingEntry(ctx, client); err != nil {
		s.Error("Failure on reading a non-existing key in boot lockbox: ", err)
	}

	if err := testWriteReadConsistency(ctx, client); err != nil {
		s.Error("Inconsistent value retrieved after writing to boot lockbox: ", err)
	}

	// Making boot lockbox read-only by starting a new chrome instance remotely
	if _, err := login(ctx, client); err != nil {
		s.Fatal("Unable to login to chrome: ", err)
	}

	if err := testWriteFailureAfterLogin(ctx, client); err != nil {
		s.Error("Writing to boot lockbox should fail after login: ", err)
	}

	if cl, err = rebootAndReconnect(ctx, s); err != nil {
		s.Fatal("Failed to reboot and reconnect to boot lockbox service")
	}
	defer cl.Close(ctx)

	// Connecting again after reboot to restore original boot lockbox value
	client = security.NewBootLockboxServiceClient(cl.Conn)
	if err := restoreTestKeyValue(ctx, client); err != nil {
		s.Error("Unable to restore test key value in boot lockbox: ", err)
	}
}

// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/golang/protobuf/proto"

	"chromiumos/policy/chromium/policy/enterprise_management_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash/ashproc"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/procutil"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/session/ownership"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UserPolicyKeys,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verifies that, after policy is pushed, the user policy key winds up stored in the right place",
		Contacts: []string{
			"mnissler@chromium.org", // session_manager owner
			"hidehiko@chromium.org", // Tast port author
		},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"testcert.p12"},
		Attr:         []string{"group:mainline"},
	})
}

func UserPolicyKeys(ctx context.Context, s *testing.State) {
	const (
		testUser = "test@foo.com"
		testPass = "test_password"
	)
	privKey, err := session.ExtractPrivKey(s.DataPath("testcert.p12"))
	if err != nil {
		s.Fatal("Failed to parse PKCS #12 file: ", err)
	}

	testDesc := ownership.UserPolicyDescriptor(testUser)
	userHash, err := cryptohome.UserHash(ctx, testUser)
	if err != nil {
		s.Fatalf("Failed to find user hash for %s: %v", testUser, err)
	}
	keyFile := filepath.Join("/run/user_policy", userHash, "policy.pub")

	readable := func(fi os.FileInfo, uid, gid uint32) (bool, error) {
		perm := fi.Mode().Perm()
		st, ok := fi.Sys().(*syscall.Stat_t)
		if st == nil || !ok {
			return false, errors.New("failed to find uid/gid")
		}
		if st.Uid == uid {
			return (perm & 0400) != 0, nil
		}
		if st.Gid == gid {
			return (perm & 0040) != 0, nil
		}
		return (perm & 0004) != 0, nil
	}

	executable := func(fi os.FileInfo, uid, gid uint32) (bool, error) {
		perm := fi.Mode().Perm()
		st, ok := fi.Sys().(*syscall.Stat_t)
		if st == nil || !ok {
			return false, errors.New("failed to find uid/gid")
		}
		if st.Uid == uid {
			return (perm & 0100) != 0, nil
		}
		if st.Gid == gid {
			return (perm & 0010) != 0, nil
		}
		return (perm & 0001) != 0, nil
	}

	verifyKeyFile := func(p string) error {
		fi, err := os.Stat(p)
		if err != nil {
			return errors.Wrapf(err, "failed to stat %s", p)
		}

		if !fi.Mode().IsRegular() {
			return errors.Errorf("%s is not a regular file", p)
		}
		if ok, err := readable(fi, sysutil.ChronosUID, sysutil.ChronosGID); err != nil {
			return errors.Wrapf(err, "failed to check readability of %s", p)
		} else if !ok {
			return errors.Errorf("%s is unreadable by chronos", p)
		}

		// Ensure parent directories are executable by chronos.
		for {
			p = filepath.Dir(p)
			di, err := os.Stat(p)
			if err != nil {
				return errors.Wrapf(err, "failed to stat %s", p)
			}

			if ok, err := executable(di, sysutil.ChronosUID, sysutil.ChronosGID); err != nil {
				return errors.Wrapf(err, "failed to check executability of %s", p)
			} else if !ok {
				return errors.Errorf("%s is unexecutable by chronos", p)
			}
			if p == "/" {
				break
			}
		}
		return nil
	}

	policy, err := session.BuildPolicy("", privKey, nil, &enterprise_management_proto.ChromeDeviceSettingsProto{})
	if err != nil {
		s.Fatal("Failed to build test policy data: ", err)
	}

	if err := session.SetUpDevice(ctx); err != nil {
		s.Fatal("Failed to reset device ownership: ", err)
	}

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to create session_manager binding: ", err)
	}
	if err := session.PrepareChromeForPolicyTesting(ctx, sm); err != nil {
		s.Fatal("Failed to prepare Chrome for testing: ", err)
	}

	// Create clean vault for the test user, and start the session.
	if err = cryptohome.RemoveVault(ctx, testUser); err != nil {
		s.Fatal("Failed to remove vault: ", err)
	}
	if err = cryptohome.CreateVault(ctx, testUser, testPass); err != nil {
		s.Fatal("Failed to create vault: ", err)
	}
	if err := sm.StartSession(ctx, testUser, ""); err != nil {
		s.Fatalf("Failed to start session for %s: %v", testUser, err)
	}

	// No policy stored yet.
	if ret, err := sm.RetrievePolicyEx(ctx, testDesc); err != nil {
		s.Fatalf("Failed to retrieve policy for %s: %v", testUser, err)
	} else if !proto.Equal(ret, &enterprise_management_proto.PolicyFetchResponse{}) {
		s.Fatal("Unexpected policy is fetched for ", testUser)
	}
	if _, err := os.Stat(keyFile); err == nil {
		s.Fatalf("%s exists unexpectedly", keyFile)
	} else if !os.IsNotExist(err) {
		s.Fatalf("Failed to stat %s: %v", keyFile, err)
	}

	// Now store a policy.
	if err := sm.StorePolicyEx(ctx, testDesc, policy); err != nil {
		s.Fatal("Failed to store user policy: ", err)
	}

	// The policy key file should have been created now.
	if err := verifyKeyFile(keyFile); err != nil {
		s.Fatal("Failed to verify created key: ", err)
	}

	oldProc, err := ashproc.Root()
	if err != nil {
		s.Fatal("Failed to find Chrome: ", err)
	}
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart ui: ", err)
	}

	// The actual deletion is done in the session_manager's Chrome setup
	// code. Thus wait for the Chrome reboot, which should be after the
	// setup.
	if err := procutil.WaitForTerminated(ctx, oldProc, 30*time.Second); err != nil {
		s.Fatal("Failed to wait for chrome shutdown: ", err)
	}
	if _, err := ashproc.WaitForRoot(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to wait for chrome to restart: ", err)
	}

	if _, err := os.Stat(keyFile); err == nil {
		s.Fatalf("%s exists unexpectedly", keyFile)
	} else if !os.IsNotExist(err) {
		s.Fatalf("Failed to stat %s: %v", keyFile, err)
	}

	// Starting a new session will restore the key that was previously
	// stored. Reconnect to the session_manager, because the restart
	// killed it.
	if err := cryptohome.CreateVault(ctx, testUser, testPass); err != nil {
		s.Fatal("Failed to mount vault: ", err)
	}
	if err := sm.StartSession(ctx, testUser, ""); err != nil {
		s.Fatalf("Failed to start session for %s: %v", testUser, err)
	}

	// The policy key file should have been restored now.
	if err := verifyKeyFile(keyFile); err != nil {
		s.Fatal("Failed to verify restored key: ", err)
	}
}

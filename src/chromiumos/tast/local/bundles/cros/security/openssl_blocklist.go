// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OpenSSLBlocklist,
		Desc: "Verifies that OpenSSL certificate blocklisting works",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"chromeos-security@google.com",
		},
		Data: []string{
			"openssl_blocklist_ca.pem",
			"openssl_blocklist_cert.key",
			"openssl_blocklist_cert.pem",
			"openssl_blocklist_bogus_blocklist",
			"openssl_blocklist_serial_blocklist",
			"openssl_blocklist_sha1_blocklist",
			"openssl_blocklist_sha256_blocklist",
		},
		Attr: []string{"group:mainline"},
	})
}

func OpenSSLBlocklist(ctx context.Context, s *testing.State) {
	var (
		caPEM          = s.DataPath("openssl_blocklist_ca.pem")
		certKey        = s.DataPath("openssl_blocklist_cert.key")
		certPEM        = s.DataPath("openssl_blocklist_cert.pem")
		nullBlocklist  = "/dev/null"
		bogusBlocklist = s.DataPath("openssl_blocklist_bogus_blocklist")
	)
	blocklists := []string{
		s.DataPath("openssl_blocklist_serial_blocklist"),
		s.DataPath("openssl_blocklist_sha1_blocklist"),
		s.DataPath("openssl_blocklist_sha256_blocklist"),
	}

	// verify runs "openssl verify" against the cert while using the supplied blocklist.
	verify := func(blocklist string, dumpOnFail bool) error {
		cmd := testexec.CommandContext(ctx, "openssl", "verify", "-CAfile", caPEM, certPEM)
		cmd.Env = append(os.Environ(), "OPENSSL_BLOCKLIST_PATH="+blocklist)
		err := cmd.Run()
		if err != nil && dumpOnFail {
			cmd.DumpLog(ctx)
		}
		return err
	}

	s.Log("Verifying blocklists")
	if err := verify(nullBlocklist, true); err != nil {
		s.Fatal("Cert does not verify normally: ", err)
	}
	if err := verify(bogusBlocklist, true); err != nil {
		s.Fatal("Cert does not verify with non-empty blocklist: ", err)
	}
	for _, bl := range blocklists {
		if err := verify(bl, false); err == nil {
			s.Error("Cert unexpectedly verified with ", filepath.Base(bl))
		}
	}

	const port = 4433
	s.Log("Starting openssl s_server on port ", port)
	srvCmd := testexec.CommandContext(ctx, "openssl", "s_server", "-www",
		"-CAfile", caPEM, "-cert", certPEM, "-key", certKey, "-port", strconv.Itoa(port))
	if err := srvCmd.Start(); err != nil {
		defer srvCmd.DumpLog(ctx)
		s.Fatal("Failed to start openssl server: ", err)
	}
	defer func() {
		srvCmd.Kill()
		srvCmd.Wait()
	}()

	// fetch uses curl with the blocklist at the supplied path to connect to the server.
	fetch := func(ctx context.Context, blocklist string) error {
		cmd := testexec.CommandContext(ctx, "curl", "--cacert", caPEM,
			fmt.Sprintf("https://127.0.0.1:%d/", port), "-o", "/dev/null")
		cmd.Env = []string{"OPENSSL_BLOCKLIST_PATH=" + blocklist}
		return cmd.Run()
	}

	s.Log("Waiting for server to be ready")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return fetch(ctx, nullBlocklist)
	}, nil); err != nil {
		s.Fatal("Failed waiting for server to be ready: ", err)
	}

	for _, bl := range blocklists {
		s.Log("Connecting to server using ", filepath.Base(bl))
		if err := fetch(ctx, bl); err == nil {
			s.Error("Connection unexpectedly succeeded using ", filepath.Base(bl))
		}
	}
}

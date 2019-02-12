// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OpenSSLBlacklist,
		Desc: "Verifies that OpenSSL certificate blacklisting works",
		Contacts: []string{
			"ellyjones@chromium.org", // original Autotest author
			"derat@chromium.org",     // Tast port author
		},
		Attr: []string{"informational"},
		Data: []string{
			"openssl_blacklist_ca.pem",
			"openssl_blacklist_cert.key",
			"openssl_blacklist_cert.pem",
			"openssl_blacklist_bogus_blacklist",
			"openssl_blacklist_serial_blacklist",
			"openssl_blacklist_sha1_blacklist",
			"openssl_blacklist_sha256_blacklist",
		},
	})
}

func OpenSSLBlacklist(ctx context.Context, s *testing.State) {
	var (
		caPEM          = s.DataPath("openssl_blacklist_ca.pem")
		certKey        = s.DataPath("openssl_blacklist_cert.key")
		certPEM        = s.DataPath("openssl_blacklist_cert.pem")
		nullBlacklist  = "/dev/null"
		bogusBlacklist = s.DataPath("openssl_blacklist_bogus_blacklist")
	)
	blacklists := []string{
		s.DataPath("openssl_blacklist_serial_blacklist"),
		s.DataPath("openssl_blacklist_sha1_blacklist"),
		s.DataPath("openssl_blacklist_sha256_blacklist"),
	}

	// verify runs "openssl verify" against the cert while using the supplied blacklist.
	verify := func(blacklist string) error {
		cmd := testexec.CommandContext(ctx, "openssl", "verify", "-CAfile", caPEM, certPEM)
		cmd.Env = []string{"OPENSSL_BLACKLIST_PATH=" + blacklist}
		return cmd.Run()
	}

	s.Log("Verifying blacklists")
	if err := verify(nullBlacklist); err != nil {
		s.Fatal("Cert does not verify normally")
	}
	if err := verify(bogusBlacklist); err != nil {
		s.Fatal("Cert does not verify with non-empty blacklist")
	}
	for _, bl := range blacklists {
		if err := verify(bl); err == nil {
			s.Error("Cert unexpectedly verified with ", filepath.Base(bl))
		}
	}

	const port = 4433
	s.Log("Starting openssl s_server on port ", port)
	srvCmd := testexec.CommandContext(ctx, "openssl", "s_server", "-www",
		"-CAfile", caPEM, "-cert", certPEM, "-key", certKey, "-port", strconv.Itoa(port))
	if err := srvCmd.Start(); err != nil {
		s.Fatal("Failed to start openssl server: ", err)
	}
	defer func() {
		srvCmd.Kill()
		srvCmd.Wait()
	}()

	// fetch uses curl with the blacklist at the supplied path to connect to the server.
	fetch := func(ctx context.Context, blacklist string) error {
		cmd := testexec.CommandContext(ctx, "curl", "--cacert", caPEM,
			fmt.Sprintf("https://127.0.0.1:%d/", port))
		cmd.Env = []string{"OPENSSL_BLACKLIST_PATH=" + blacklist}
		return cmd.Run()
	}

	s.Log("Waiting for server to be ready")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return fetch(ctx, nullBlacklist)
	}, nil); err != nil {
		s.Fatal("Failed waiting for server to be ready: ", err)
	}

	for _, bl := range blacklists {
		s.Log("Connecting to server using ", filepath.Base(bl))
		if err := fetch(ctx, bl); err == nil {
			s.Error("Connection unexpectedly succeeded using ", filepath.Base(bl))
		}
	}
}

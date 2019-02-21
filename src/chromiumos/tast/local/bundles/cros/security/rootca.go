// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RootCA,
		Desc: "Ensures that the built-in root CAs matches the whitelist",
		Contacts: []string{
			"jorgelo@chromium.org",  // Security team
			"ejcaruso@chromium.org", // Tast port author
			"chromeos-security@google.com",
		},
		Attr: []string{"informational"},
	})
}

func RootCA(ctx context.Context, s *testing.State) {
	getNssCerts := func() (certs map[string]string, err error) {
		const certUtilPath = "/usr/local/bin/certutil"
		const modUtilPath = "/usr/local/bin/modutil"

		dir, err := ioutil.TempDir("", "tast.security.RootCA.")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(dir)

		nssLibs, err := filepath.Glob("/usr/lib*/libnssckbi.so")
		if err != nil {
			return nil, err
		}

		if len(nssLibs) == 0 {
			return make(map[string]string), nil
		} else if len(nssLibs) > 1 {
			s.Log("Found multiple copies of libnssckbi.so")
		}

		// Create new empty cert DB.
		child := exec.CommandContext(ctx, certUtilPath, "-N", "-d", dir, "-f", "--empty-password")
		if err := child.Run(); err != nil {
			return nil, err
		}

		// Add the certs found in the compiled NSS shlib to a new module in the DB.
		child = exec.CommandContext(ctx, modUtilPath,
			"-add", "testroots", "-libfile", nssLibs[0], "-dbdir", dir, "-force")
		if err := child.Run(); err != nil {
			return nil, err
		}

		child = exec.CommandContext(ctx, modUtilPath, "-list", "-dbdir", dir)
		output, err := child.Output()
		if err != nil {
			return nil, err
		}
		if !regexp.MustCompile(`2\. testroots`).Match(output) {
			s.Fatal("testroots PKCS#11 module did not appear in the db")
		}

		// Dump out the list of root certs.
		child = exec.CommandContext(ctx, certUtilPath, "-L", "-d", dir, "-h", "all")
		output, err = child.Output()
		if err != nil {
			return nil, err
		}

		parseByRegexp := func(b []byte, re string) [][]byte {
			matches := make([][]byte, 0)
			for _, match := range regexp.MustCompile(re).FindAllSubmatch(b, -1) {
				matches = append(matches, match[1])
			}
			return matches
		}

		certMap := make(map[string]string)
		for _, certBytes := range parseByRegexp(output, `Builtin Object Token:(.+?)\s+C,.?,.?`) {
			child = exec.CommandContext(ctx, certUtilPath, "-L", "-d", dir, "-n",
				fmt.Sprintf("Builtin Object Token:%s", certBytes))
			output, err = child.CombinedOutput()
			if err != nil {
				s.Log("Could not find certificate")
				continue
			}

			cert := string(certBytes)
			for _, fp := range parseByRegexp(output, `Fingerprint \(SHA1\):\n\s+(\b[:\w]+)\b`) {
				certMap[string(fp)] = cert
			}
		}

		return certMap, nil
	}

	opensslCertGlob := "/etc/ssl/certs/" + strings.Repeat("[0-9a-f]", 8) + ".*"
	getOpensslCerts := func() (certs map[string]string, err error) {
		const opensslPath = "/usr/bin/openssl"
		fingerprintCmdArgs := []string{"x509", "-fingerprint", "-issuer", "-noout", "-in"}

		certPaths, err := filepath.Glob(opensslCertGlob)
		if err != nil {
			return nil, err
		}

		certMap := make(map[string]string)

		for _, certPath := range certPaths {
			child := exec.CommandContext(ctx, opensslPath, append(fingerprintCmdArgs, certPath)...)
			output, err := child.Output()
			if err != nil {
				return nil, err
			}
			lines := strings.Split(string(output), "\n")

			// The output looks like:
			//   SHA Fingerprint=<hex string>
			//   issuer= /C=US/ST=Texas/L=Houston/O=SSL Corporation/CN=SSL.com EV Root...
			fingerprint := strings.Split(lines[0], "=")[1]
			for _, field := range strings.Split(lines[1], "/") {
				// Compensate for stupidly malformed issuer fields.
				items := strings.Split(field, "=")
				if len(items) > 1 {
					if items[0] == "CN" || items[0] == "O" {
						certMap[fingerprint] = items[1]
						break
					}
				} else {
					s.Log("Malformed issuer string ", lines[1])
				}
			}

			// Check that we found a name for this fingerprint.
			_, ok := certMap[fingerprint]
			if !ok {
				return nil, errors.Errorf("Couldn't find issuer string for %v", fingerprint)
			}
		}

		return certMap, nil
	}

	certs, err := getNssCerts()
	if err != nil {
		s.Fatal("Failed to get NSS certs: ", err)
	}

	s.Logf("Found %v NSS certs", len(certs))

	certs, err = getOpensslCerts()
	if err != nil {
		s.Fatal("Failed to get OpenSSL certs: ", err)
	}

	s.Logf("Found %v OpenSSL certs", len(certs))

	// Regression test for crbug.com/202944
	certPaths, err := filepath.Glob(opensslCertGlob)
	if err != nil {
		s.Fatal("Failed to glob OpenSSL certs to check perms: ", err)
	}

	for _, certPath := range certPaths {
		stat, err := os.Stat(certPath)
		if err != nil {
			s.Fatalf("Failed to stat OpenSSL cert %q to check perms: %v", certPath, err)
		}
		st := stat.Sys().(*syscall.Stat_t)
		if st.Uid != 0 || stat.Mode().Perm() != 0644 {
			s.Errorf("Bad permissions on %q: uid %v, perms %v", certPath, st.Uid, stat.Mode().Perm())
		}
	}
}

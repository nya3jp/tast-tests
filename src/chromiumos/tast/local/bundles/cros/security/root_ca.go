// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RootCA,
		Desc: "Ensures that the built-in root CAs match the whitelist",
		Contacts: []string{
			"jorgelo@chromium.org",  // Security team
			"ejcaruso@chromium.org", // Tast port author
			"chromeos-security@google.com",
		},
		Attr: []string{"informational"},
		Data: []string{
			whitelistPath,
		},
	})
}

const (
	whitelistPath = "root_ca_whitelist.json"
)

func RootCA(ctx context.Context, s *testing.State) {
	getNSSCerts := func() (certs map[string]string, err error) {
		nssLibs, err := filepath.Glob("/usr/lib*/libnssckbi.so")
		if err != nil {
			return nil, err
		}

		if len(nssLibs) == 0 {
			return make(map[string]string), nil
		} else if len(nssLibs) > 1 {
			s.Log("Found multiple copies of libnssckbi.so: ", nssLibs)
		}

		dir, err := ioutil.TempDir("", "tast.security.RootCA.")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(dir)

		// Create new empty cert DB.
		child := testexec.CommandContext(ctx, "certutil", "-N", "-d", dir, "-f", "--empty-password")
		if err := child.Run(); err != nil {
			child.DumpLog(ctx)
			return nil, err
		}

		// Add the certs found in the compiled NSS shlib to a new module in the DB.
		child = testexec.CommandContext(ctx, "modutil",
			"-add", "testroots", "-libfile", nssLibs[0], "-dbdir", dir, "-force")
		if err := child.Run(); err != nil {
			child.DumpLog(ctx)
			return nil, err
		}

		child = testexec.CommandContext(ctx, "modutil", "-list", "-dbdir", dir)
		output, err := child.Output()
		if err != nil {
			child.DumpLog(ctx)
			return nil, err
		}
		if !regexp.MustCompile(`2\. testroots`).Match(output) {
			s.Fatal("testroots PKCS#11 module did not appear in the db")
		}

		// Dump out the list of root certs.
		child = testexec.CommandContext(ctx, "certutil", "-L", "-d", dir, "-h", "all")
		output, err = child.Output()
		if err != nil {
			child.DumpLog(ctx)
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
			child = testexec.CommandContext(ctx, "certutil", "-L", "-d", dir, "-n",
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

	openSSLCertGlob := "/etc/ssl/certs/" + strings.Repeat("[0-9a-f]", 8) + ".*"
	getOpenSSLCerts := func() (certs map[string]string, err error) {
		certPaths, err := filepath.Glob(openSSLCertGlob)
		if err != nil {
			return nil, err
		}

		certMap := make(map[string]string)
		for _, certPath := range certPaths {
			child := testexec.CommandContext(ctx, "openssl",
				"x509", "-fingerprint", "-issuer", "-noout", "-in", certPath)
			output, err := child.Output()
			if err != nil {
				child.DumpLog(ctx)
				return nil, err
			}
			lines := strings.Split(string(output), "\n")
			if len(lines) < 2 {
				s.Logf("Unexpected output from openssl: %q", output)
				continue
			}

			// The output looks like:
			//   SHA Fingerprint=<hex string>
			//   issuer= /C=US/ST=Texas/L=Houston/O=SSL Corporation/CN=SSL.com EV Root...
			fingerprint := strings.Split(lines[0], "=")[1]
			for _, field := range strings.Split(lines[1], "/") {
				// Compensate for stupidly malformed issuer fields.
				items := strings.Split(field, "=")
				if len(items) <= 1 {
					s.Logf("Malformed issuer string %q", lines[1])
					continue
				}
				if items[0] == "CN" || items[0] == "O" {
					certMap[fingerprint] = items[1]
					break
				}
			}

			// Check that we found a name for this fingerprint.
			if _, ok := certMap[fingerprint]; !ok {
				return nil, errors.Errorf("couldn't find issuer string for %v", fingerprint)
			}
		}

		return certMap, nil
	}

	parseBaseline := func(r io.Reader) (nssCerts map[string]string, openSSLCerts map[string]string) {
		d := json.NewDecoder(r)
		var certs interface{}
		if err := d.Decode(&certs); err != nil {
			s.Fatal("Couldn't parse certs baseline")
		}

		certMap, ok := certs.(map[string]interface{})
		if !ok {
			s.Fatal("Certs baseline file was malformed")
		}

		addCerts := func(certs map[string]string, certType string) {
			certDict, ok := certMap[certType]
			if !ok {
				s.Log("Did not have certs dict for type ", certType)
				return
			}
			typedDict, _ := certDict.(map[string]interface{})

			for fingerprint, v := range typedDict {
				issuer, ok := v.(string)
				if !ok {
					continue
				}
				certs[fingerprint] = issuer
			}
		}

		nssMap := make(map[string]string)
		openSSLMap := make(map[string]string)
		addCerts(nssMap, "nss")
		addCerts(nssMap, "both")
		addCerts(openSSLMap, "openssl")
		addCerts(openSSLMap, "both")
		return nssMap, openSSLMap
	}

	compareCerts := func(expected map[string]string, found map[string]string) {
		for fingerprint, issuer := range expected {
			if _, ok := found[fingerprint]; !ok {
				s.Error("Did not find expected cert with fingerprint %q (issuer %q)", fingerprint, issuer)
			}
		}
		for fingerprint, issuer := range found {
			if _, ok := expected[fingerprint]; !ok {
				s.Error("Found unexpected cert with fingerprint %q (issuer %q) ", fingerprint, issuer)
			}
		}
	}

	baseline, err := os.Open(s.DataPath(whitelistPath))
	if err != nil {
		s.Fatal("Failed to open whitelist")
	}
	defer baseline.Close()
	nssExpected, openSSLExpected := parseBaseline(bufio.NewReader(baseline))

	s.Logf("Expecting %v NSS cert(s) and %v OpenSSL certs", len(nssExpected), len(openSSLExpected))

	nssFound, err := getNSSCerts()
	if err != nil {
		s.Fatal("Failed to get NSS certs: ", err)
	}

	s.Logf("Found %v NSS cert(s)", len(nssFound))
	compareCerts(nssExpected, nssFound)

	openSSLFound, err := getOpenSSLCerts()
	if err != nil {
		s.Fatal("Failed to get OpenSSL certs: ", err)
	}

	s.Logf("Found %v OpenSSL cert(s)", len(openSSLFound))
	compareCerts(openSSLExpected, openSSLFound)

	// Regression test for crbug.com/202944
	certPaths, err := filepath.Glob(openSSLCertGlob)
	if err != nil {
		s.Fatal("Failed to glob OpenSSL certs to check perms: ", err)
	}

	for _, certPath := range certPaths {
		stat, err := os.Stat(certPath)
		if err != nil {
			s.Errorf("Failed to stat OpenSSL cert %v to check perms: %v", certPath, err)
			continue
		}
		st := stat.Sys().(*syscall.Stat_t)
		if st.Uid != 0 || stat.Mode().Perm() != 0644 {
			s.Errorf("Bad permissions on %v: uid %v, mode %s", certPath, st.Uid, stat.Mode())
		}
	}
}

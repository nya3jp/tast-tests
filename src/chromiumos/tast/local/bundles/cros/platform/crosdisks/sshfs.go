// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crosdisks

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosdisks"
	"chromiumos/tast/testing"
)

const sshDir = "/home/chronos/user/.ssh"
const authorizedKeysPath = sshDir + "/authorized_keys"
const authorizedKeysBackupPath = sshDir + "/authorized_keys.sshfsbak"
const sshHostKeyPath = "/mnt/stateful_partition/etc/ssh/ssh_host_ed25519_key.pub"

// Some discardable SSH key.
const sshPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAvKhQn82O9F+SzDTYgpI+qnCD6E6cYroLvflLp8/onYdqD1xK
ES4wDTGC68DNbS9tIo1hEjwbD79UltQT9NTmJg8DERUrQbNayYXtwxqZ2tSo1Hg5
dpAKLd3GBhwK1Eob+bNgcqEu3iIZq+QRVtlM92Uj4vBFuy8qgvGs4x+n3lACsyk8
4GZGtiFpqTPlTZ6BOEdknZpB0K3HIZ7NjZ8uD9fXJYFuUgQhQvhp1N8aZaf7JtWr
GLQ8Pwq6UYEVb8veHgLVAJ9p/5ko/WNVWf1h78v95pEHSYrQ0opcSDizbquW/1Fs
Fk6elrQcKctJ1FsXMxlWYOzN31yNxcPqT6rzhQIDAQABAoIBAAcD50OZ/DfgGfBY
ArkQQR5LYsxPqAcPzgH5dDPASnEZKPt7PhHXetfywGCN4dWujstbIIHyFDuIrNeS
+U8AX7KIml+XPu2JgtW9kjLQGWqGv+RuuAxNnONJvORbRJfSTaoCXpLEpZ6C/Btl
NrPZDsCgVS5KKv2j6lvGKtyjP7XHiXIXLvlhOJkpWRk4a1IBISP8HPt2w/bG0raD
CW6e6XYYPI4ZPwMlPRympQPGo8mVpNkhFAMHKnaN+E7HplsWXvb0daAVUeCBDVId
QSat88e7PbK2FMsinZvsCZSrHdggS+4u1h6LjMI3GO1PYEjvrMorkHz2w1KS3n1S
n4Eas50CgYEA9JR6JCauiZqJAV4azOylZaeiClkAtsK1IG98XkHyIDfn634U7o5c
6w1Uf0zwxRKx12EPQhzKiYRtp+nPirMAZHmm+gJqExakDV7uJlHNo/6qY7m1Z8I7
Ww/my1Oi5ASQ6Emyrpecvo8xTTl52Kf+l3mQk/EqitZLNWgkX5HdwTMCgYEAxXdh
/DLRDrBz7b+lYahAvBCr+VqUxWdjiarpnC/NZmXIWsI7U0nFpf3H4JzEQVdu5gdV
DKLrU8uw1dGwytgH3zA8s1VMWVg1uvVFduQk+pZeRj9ekGEHvViUEkylg3CaNCyO
2Kl72VS8W/ls5uX74mFcx9fwc9jUue807+406GcCgYAjfpTHQFHeKG4vo5+SE9nh
CdXrWIVRAKrWnTdYWouv/00KEQ8qm8CCYDneC6V5hEAI+M4FEzaVhIGBd94ly9qH
ulvwNn98a7G9OwSmzQJiBWhm9qGMAFUq3wDoiye9nagF/gQPcHNP+Gn4Qhobxi2d
gAfqYHqDEZxykL2OnRWonwKBgBvcl1+9T9ARx5mxI8WettuSQqGhTUJ5Lws6qVGX
URT0oYtkwngi/ZdJMo2XsP1DN+uO90ocJrYhFGdm+dn1F08/gCERlP86OgKSHuYC
lNEirFSfFlmqxyvJNsNKO0RLfAaGjvU1HLtygE096Ua/BoZPlIbCCjReUM2XWdHM
u3xbAoGARqG0gGpNCY7pEjSQ33TdLEXV7O0S4hJiN+IKPS2q8q8k21X7ckkPxsEG
h4dIuHzdLGZsmIXel4Mx3rvyKbboj93K3ia5rbU05keVi9duMv1PlbdQzu9mq2qu
A5CmV2fYpStHZTHsv5BcYWxkhc4aAmvUJwyAzlWEhyijwFK5wSQ=
-----END RSA PRIVATE KEY-----
`

const sshPublicKey = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC8qFCfzY" +
	"70X5LMNNiCkj6qcIPoTpxiugu9+Uunz+idh2oPXEoRLjANMYLrwM1tL" +
	"20ijWESPBsPv1SW1BP01OYmDwMRFStBs1rJhe3DGpna1KjUeDl2kAot" +
	"3cYGHArUShv5s2ByoS7eIhmr5BFW2Uz3ZSPi8EW7LyqC8azjH6feUAK" +
	"zKTzgZka2IWmpM+VNnoE4R2SdmkHQrcchns2Nny4P19clgW5SBCFC+G" +
	"nU3xplp/sm1asYtDw/CrpRgRVvy94eAtUAn2n/mSj9Y1VZ/WHvy/3mk" +
	"QdJitDSilxIOLNuq5b/UWwWTp6WtBwpy0nUWxczGVZg7M3fXI3Fw+pP" +
	"qvOF root@localhost\n"

// RunSSHFSTests runs sshfs-related tests.
func RunSSHFSTests(ctx context.Context, s *testing.State) {
	cd, err := crosdisks.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect CrosDisks D-Bus service: ", err)
	}
	defer cd.Close()

	// Backup current keys for chronos if any.
	if err := os.Rename(authorizedKeysPath, authorizedKeysBackupPath); err == nil {
		defer os.Rename(authorizedKeysBackupPath, authorizedKeysPath)
	}

	// Write predefined key.
	if os.Mkdir(sshDir, 0755) != nil {
		defer os.Remove(sshDir)
		if err := os.Chown(sshDir, chronosUID, chronosGID); err != nil {
			s.Fatalf("Could not set correct owner of directory %q: %v", sshDir, err)
		}
	}
	if err := ioutil.WriteFile(authorizedKeysPath, []byte(sshPublicKey), 0600); err != nil {
		s.Fatalf("Could not write file %q: %v", authorizedKeysPath, err)
	}
	defer os.Remove(authorizedKeysPath)
	if err := os.Chown(authorizedKeysPath, chronosUID, chronosGID); err != nil {
		s.Fatalf("Could not set correct owner of file %q: %v", authorizedKeysPath, err)
	}

	// We are using the ssh server running on the same DUT to verify mounting.
	// Read host's identification.
	data, err := ioutil.ReadFile(sshHostKeyPath)
	if err != nil {
		s.Fatal("Could not read the host identification: ", err)
	}
	identity := base64.StdEncoding.EncodeToString([]byte(sshPrivateKey))
	knownHosts := base64.StdEncoding.EncodeToString([]byte("localhost " + string(data)))

	s.Run(ctx, "mount", func(ctx context.Context, state *testing.State) {
		const src = "sshfs://chronos@localhost:"
		const mnt = "/media/fuse/chronos@localhost:"
		opts := []string{fmt.Sprintf("IdentityBase64=%s", identity), fmt.Sprintf("UserKnownHostsBase64=%s", knownHosts)}
		if err := WithMountDo(ctx, cd, src, "sshfs", opts, func(ctx context.Context, mountPath string, readOnly bool) error {
			if mountPath != mnt {
				return errors.Errorf("mounth path mismatch: got %q; expected %q", mountPath, mnt)
			}

			if readOnly {
				return errors.Errorf("unexpected read-only flag for %q: got %v; want false", mountPath, readOnly)
			}

			expectedFile := filepath.Join(mountPath, ".ssh/authorized_keys")
			if _, err := os.Stat(expectedFile); err != nil {
				return errors.Wrapf(err, "could not stat file %q that should have existed", expectedFile)
			}
			return nil
		}); err != nil {
			s.Fatal("Test case failed: ", err)
		}
	})
	// TODO(crbug.com/898341): Worth adding tests to verify behavior on server shutting down/disconnecting,
	// but it'd need to be a separate instance of sshd then.
}

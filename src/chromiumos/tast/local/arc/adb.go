// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	adbAddr = "100.115.92.2:5555"

	adbHome               = "/tmp/adb_home"
	testPrivateKeyPath    = "/tmp/adb_home/test_key"
	androidPublicKeysPath = "/data/misc/adb/adb_keys"

	// Generated with adb keygen.
	testPrivateKey = `-----BEGIN PRIVATE KEY-----
MIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQCnHNzujonYRLoI
F2pyJX1SSrqmiT/3rTRCP1X0pj1V/sPGwgvIr+3QjZehLUGRQL0wneBNXd6EVrST
drO4cOPwSxRJjCf+/PtS1nwkz+o/BGn5yhNppdSro7aPoQxEVM8qLtN5Ke9tx/zE
ggxpF8D3XBC6Los9lAkyesZI6xqXESeofOYu3Hndzfbz8rAjC0X+p6Sx561Bt1dn
T7k2cP0mwWfITjW8tAhzmKgL4tGcgmoLhMHl9JgScFBhW2Nd0QAR4ACyVvryJ/Xa
2L6T2YpUjqWEDbiJNEApFb+m+smIbyGz0H/Kj9znoRs84z3/8rfyNQOyf7oqBpr2
52XG4totAgMBAAECggEARisKYWicXKDO9CLQ4Uj4jBswsEilAVxKux5Y+zbqPjeR
AN3tkMC+PHmXl2enRlRGnClOS24ExtCZVenboLBWJUmBJTiieqDC7o985QAgPYGe
9fFxoUSuPbuqJjjbK73olq++v/tpu1Djw6dPirkcn0CbDXIJqTuFeRqwM2H0ckVl
mVGUDgATckY0HWPyTBIzwBYIQTvAYzqFHmztcUahQrfi9XqxnySI91no8X6fR323
R8WQ44atLWO5TPCu5JEHCwuTzsGEG7dEEtRQUxAsH11QC7S53tqf10u40aT3bXUh
XV62ol9Zk7h3UrrlT1h1Ae+EtgIbhwv23poBEHpRQQKBgQDeUJwLfWQj0xHO+Jgl
gbMCfiPYvjJ9yVcW4ET4UYnO6A9bf0aHOYdDcumScWHrA1bJEFZ/cqRvqUZsbSsB
+thxa7gjdpZzBeSzd7M+Ygrodi6KM/ojSQMsen/EbRFerZBvsXimtRb88NxTBIW1
RXRPLRhHt+VYEF/wOVkNZ5c2eQKBgQDAbwNkkVFTD8yQJFxZZgr1F/g/nR2IC1Yb
ylusFztLG998olxUKcWGGMoF7JjlM6pY3nt8qJFKek9bRJqyWSqS4/pKR7QTU4Nl
a+gECuD3f28qGFgmay+B7Fyi9xmBAsGINyVxvGyKH95y3QICw1V0Q8uuNwJW2feo
3+UD2/rkVQKBgFloh+ljC4QQ3gekGOR0rf6hpl8D1yCZecn8diB8AnVRBOQiYsX9
j/XDYEaCDQRMOnnwdSkafSFfLbBrkzFfpe6viMXSap1l0F2RFWhQW9yzsvHoB4Br
W7hmp73is2qlWQJimIhLKiyd3a4RkoidnzI8i5hEUBtDsqHVHohykfDZAoGABNhG
q5eFBqRVMCPaN138VKNf2qon/i7a4iQ8Hp8PHRr8i3TDAlNy56dkHrYQO2ULmuUv
Erpjvg5KRS/6/RaFneEjgg9AF2R44GrREpj7hP+uWs72GTGFpq2+v1OdTsQ0/yr0
RGLMEMYwoY+y50Lnud+jFyXHZ0xhkdzhNTGqpWkCgYBigHVt/p8uKlTqhlSl6QXw
1AyaV/TmfDjzWaNjmnE+DxQfXzPi9G+cXONdwD0AlRM1NnBRN+smh2B4RBeU515d
x5RpTRFgzayt0I4Rt6QewKmAER3FbbPzaww2pkfH1zr4GJrKQuceWzxUf46K38xl
yee+dcuGhs9IGBOEEF7lFA==
-----END PRIVATE KEY-----
`
	testPublicKey = "QAAAAFt6z0Mt2uLGZef2mgYqun+yAzXyt/L/PeM8G6Hn3I/Kf9CzIW+IyfqmvxUpQDSJuA2EpY5UitmTvtja9Sfy+layAOARANFdY1thUHASmPTlwYQLaoKc0eILqJhzCLS8NU7IZ8Em/XA2uU9nV7dBreexpKf+RQsjsPLz9s3dedwu5nyoJxGXGutIxnoyCZQ9iy66EFz3wBdpDILE/Mdt7yl50y4qz1REDKGPtqOr1KVpE8r5aQQ/6s8kfNZS+/z+J4xJFEvw43C4s3aTtFaE3l1N4J0wvUCRQS2hl43Q7a/IC8LGw/5VPab0VT9CNK33P4mmukpSfSVyahcIukTYiY7u3Byn0Nc9qhPPbSQYNQiofN7w91BWzW46V8CgWzBCKZoKhF7YmTdAm48qmaV0rqMGaf1AtRz5QY0a47seRYCgk9lMx7BeMgIuAZDmYPsUG+mAG+IiQYfvJMIEMBowtc8IlfZv9A7bwLKcs4rRhxFdCzJ7odPgFdgUv7MEAYF+HhnQg6DYEhoqe7YkB98Pb8VbU4f/ZTNkHYtIOxMIb53saW09zop5MlQrR6E7hBeZ5FwMNOK7+yc20ulUlqq38iB6QoHx7lli8dfGpD47J1ETHw7m9uAuxMu75MD4bIxYgmj2Ud1TvmWqXtmg75+E+B1I3osGcw9a2Qxo2ypV1Nkq8b1lmgEAAQA= root@localhost"
)

// setUpADBAuth sets up public key authentication of ADB.
func setUpADBAuth(ctx context.Context) error {
	// Wait for /data to be mounted.
	// ro.data_mounted is set by arcbootcontinue command invoked by arc-setup
	// when an ARC-enabled user session is started.
	// This runs in the background, so don't report timing information.
	if err := waitProp(ctx, "ro.data_mounted", "1", noReportTiming); err != nil {
		return errors.Wrap(err, "failed to wait for /data to be mounted")
	}

	// Set up the ADB home directory in Chrome OS side.
	if err := os.MkdirAll(adbHome, 0755); err != nil {
		return err
	}
	if err := ioutil.WriteFile(testPrivateKeyPath, []byte(testPrivateKey), 0600); err != nil {
		return errors.Wrap(err, "failed installing ADB private key")
	}

	// Install the ADB public key in Android side.
	if err := directWriteFile(ctx, androidPublicKeysPath, []byte(testPublicKey)); err != nil {
		return errors.Wrap(err, "failed installing ADB public key")
	}
	if err := BootstrapCommand(ctx, "/system/bin/chown", "shell", androidPublicKeysPath).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to chown ADB public key")
	}
	if err := BootstrapCommand(ctx, "/system/bin/restorecon", androidPublicKeysPath).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to restorecon ADB public key")
	}

	// Restart adbd to load the newly installed public key.
	if err := restartADBDaemon(ctx); err != nil {
		return errors.Wrap(err, "failed to restart adbd")
	}

	// Restart local ADB server to use the newly installed private key.
	// We do not use adb kill-server since it is unreliable (crbug.com/855325).
	testexec.CommandContext(ctx, "killall", "--quiet", "--wait", "-KILL", "adb").Run()
	if err := adbCommand(ctx, "start-server").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed starting ADB local server")
	}

	return nil
}

// connectADB connects to the remote ADB daemon.
// After this function returns successfully, we can assume that ADB connection is ready.
func connectADB(ctx context.Context) error {
	defer timing.Start(ctx, "connect_adb").End()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := adbCommand(ctx, "connect", adbAddr).Output()
		if err != nil {
			return err
		}
		msg := strings.SplitN(string(out), "\n", 2)[0]
		if !strings.HasPrefix(msg, "connected to ") {
			return errors.Errorf("adb connect failed (adb output: %q)", msg)
		}
		return nil
	}, nil); err != nil {
		return err
	}

	return adbCommand(ctx, "wait-for-device").Run(testexec.DumpLogOnError)
}

// Install installs an APK file to the Android system.
func (a *ARC) Install(ctx context.Context, path string) error {
	if err := a.Command(ctx, "settings", "put", "global", "verifier_verify_adb_installs", "0").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed disabling verifier_verify_adb_installs")
	}

	out, err := adbCommand(ctx, "install", "-r", "-d", path).Output(testexec.DumpLogOnError)
	if err != nil {
		return err
	}

	// "Success" is the only possible positive result. See runInstall() here:
	// https://android.googlesource.com/platform/frameworks/base/+/refs/heads/pie-release/services/core/java/com/android/server/pm/PackageManagerShellCommand.java
	matched, err := regexp.Match("^Success", out)
	if err != nil {
		return err
	}
	if !matched {
		testing.ContextLogf(ctx, "Install output: %q", string(out))
		return errors.Errorf("failed to install %v", path)
	}
	return nil
}

// adbCommand runs an ADB command with appropriate environment variables.
func adbCommand(ctx context.Context, arg ...string) *testexec.Cmd {
	cmd := testexec.CommandContext(ctx, "adb", arg...)
	cmd.Env = append(
		os.Environ(),
		"ADB_VENDOR_KEYS="+testPrivateKeyPath,
		// adb expects $HOME to be writable.
		"HOME="+adbHome)
	return cmd
}

// restartADBDaemon restarts adbd.
func restartADBDaemon(ctx context.Context) error {
	// adbd is not running by default on -user images, so set (persist).sys.usb.config
	// to enable it.
	const config = "mtp,adb"
	setProp(ctx, "persist.sys.usb.config", config)
	setProp(ctx, "sys.usb.config", config)
	if err := waitProp(ctx, "sys.usb.state", config, noReportTiming); err != nil {
		return err
	}
	return setProp(ctx, "ctl.restart", "adbd")
}

func setProp(ctx context.Context, name, value string) error {
	return BootstrapCommand(ctx, "/system/bin/setprop", name, value).Run(testexec.DumpLogOnError)
}

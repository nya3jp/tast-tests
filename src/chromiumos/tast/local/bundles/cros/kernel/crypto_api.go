// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptoAPI,
		Desc: "Verifies that the crypto user API can't be used to load arbitrary modules, using the kernel module test_module",
		Contacts: []string{
			"briannorris@chromium.org", // Original test author
			"chromeos-kernel@google.com",
			"oka@chromium.org", // Tast port author
		},
		Attr: []string{"informational"},
	})
}

func CryptoAPI(ctx context.Context, s *testing.State) {
	// TODO(oka): Use software deps to do the equivalent of test_is_valid in the original test to skip the test.
	if err := tryLoadMod(ctx, "test_module"); err != nil {
		s.Error("Failed tryLoadMod: ", err)
	}
}

// tryLoadMod tries to load a (non-crypto) module using the crypto UAPI.
func tryLoadMod(ctx context.Context, module string) error {
	loaded, err := moduleIsLoaded(ctx, module)
	if err != nil {
		return errors.Wrap(err, "failed on test preparation")
	}
	if loaded {
		if err := unloadModule(ctx, module); err != nil {
			return errors.Wrap(err, "failed on test preparation")
		}
	}

	if err := cryptoLoadMod(module); err != nil {
		return err
	}

	loaded, err = moduleIsLoaded(ctx, module)
	if err != nil {
		return errors.Wrap(err, "failed checking test result")
	}
	if loaded {
		if err := unloadModule(ctx, module); err != nil {
			return errors.Wrap(err, "failed checking test result")
		}
		return errors.Errorf("%s was unexpectedly loaded using crypto UAPI", module)
	}
	return nil
}

func cryptoLoadMod(module string) error {
	sock, err := unix.Socket(unix.AF_ALG, unix.SOCK_SEQPACKET, 0)
	if err != nil {
		return errors.Wrapf(err, "cryptoLoadMod(%q)", module)
	}
	if err := unix.Bind(sock, &unix.SockaddrALG{
		Type: "hash",
		Name: module,
	}); err != nil {
		return errors.Wrapf(err, "cryptoLoadMod(%q)", module)
	}
	return unix.Close(sock)
}

func moduleIsLoaded(ctx context.Context, module string) (bool, error) {
	_, err := getModuleLine(ctx, module)
	if err == nil {
		return true, nil
	} else if isModuleNotFound(err) {
		return false, nil
	} else {
		return false, errors.Wrap(err, "moduleIsLoaded")
	}
}

func isModuleNotFound(err error) bool {
	_, ok := err.(*moduleNotFound)
	return ok
}

type moduleNotFound struct {
	m string
}

func (e *moduleNotFound) Error() string {
	return fmt.Sprintf("%s not found", e.m)
}

func getModuleLine(ctx context.Context, module string) (string, error) {
	b, err := testexec.CommandContext(ctx, "/bin/lsmod").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrapf(err, "getModuleLine(%s)", module)
	}
	modules := strings.Split(string(b), "\n")
	for _, m := range modules {
		if strings.HasPrefix(m, strings.Replace(module, "-", "_", -1)+" ") {
			return m, nil
		}
	}
	return "", &moduleNotFound{module}
}

// unloadModule removes a module, recursively handling dependencies.
// If even then it's not possible to remove one of the modules, returns an error.
func unloadModule(ctx context.Context, module string) error {
	line, err := getModuleLine(ctx, module)
	if err != nil {
		if isModuleNotFound(err) {
			// Module is already unloaded.
			return nil
		}
		return err
	}
	parts := strings.Fields(line)
	var submodules []string
	if len(parts) == 4 {
		submodules = strings.Split(parts[3], ",")
	}
	for _, sm := range submodules {
		if err := unloadModule(ctx, sm); err != nil {
			return err
		}
	}

	testing.ContextLog(ctx, "Unloading ", module)
	return testexec.CommandContext(ctx, "/sub/modprobe", "-r", module).Run(testexec.DumpLogOnError)
}

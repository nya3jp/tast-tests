// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"context"
	"strings"
	"syscall"

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
	modDeps, err := readModuleDeps(ctx)
	if err != nil {
		s.Fatal("Failed to read modules: ", err)
	}
	const module = "test_module"
	if _, ok := modDeps[module]; ok {
		if err := unloadModule(ctx, modDeps, module); err != nil {
			s.Fatal("Failed on test preparation: ", err)
		}
	}

	if err := cryptoLoadMod(module); err == nil {
		s.Error("Unexpected success on loading module (ENOENT expected)")
	} else if err != syscall.ENOENT {
		s.Error("Unexpected error on loading module (ENOENT expected): ", err)
	}

	modDeps, err = readModuleDeps(ctx)
	if err != nil {
		s.Fatal("Failed to read modules: ", err)
	}
	if _, ok := modDeps[module]; ok {
		s.Errorf("%s was unexpectedly loaded using crypto UAPI", module)
		if err := unloadModule(ctx, modDeps, module); err != nil {
			s.Fatal("Failed to unload module: ", err)
		}
	}
}

func cryptoLoadMod(module string) error {
	sock, err := unix.Socket(unix.AF_ALG, unix.SOCK_SEQPACKET, 0)
	if err != nil {
		return errors.Wrap(err, "failed to create socket for loading mod")
	}
	defer unix.Close(sock)
	return unix.Bind(sock, &unix.SockaddrALG{
		Type: "hash",
		Name: module,
	})
}

// readModuleDeps returns a map from module name to its dependents.
func readModuleDeps(ctx context.Context) (map[string][]string, error) {
	b, err := testexec.CommandContext(ctx, "lsmod").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read modules")
	}
	mods := strings.Split(strings.TrimSpace(string(b)), "\n")
	// Sanity check.
	if !strings.HasPrefix(mods[0], "Module ") {
		return nil, errors.Errorf("unexpected header %q", mods[0])
	}

	res := make(map[string][]string)
	for _, m := range mods[1:] {
		if fs := strings.Fields(m); len(fs) == 3 {
			res[fs[0]] = nil
		} else if len(fs) == 4 {
			res[fs[0]] = strings.Split(fs[3], ",")
		} else {
			return nil, errors.Errorf("malformed module line %q", m)
		}
	}
	return res, nil
}

// unloadModule removes a module, recursively handling dependents.
// If even then it's not possible to remove one of the modules, returns an error.
func unloadModule(ctx context.Context, deps map[string][]string, module string) error {
	mods, ok := deps[module]
	if !ok {
		// Module is already unloaded.
		return nil
	}
	for _, m := range mods {
		if err := unloadModule(ctx, deps, m); err != nil {
			return err
		}
	}

	testing.ContextLog(ctx, "Unloading ", module)
	return testexec.CommandContext(ctx, "modprobe", "-r", module).Run(testexec.DumpLogOnError)
}

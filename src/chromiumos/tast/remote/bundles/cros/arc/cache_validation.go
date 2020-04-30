// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CacheValidation,
		Desc: "CacheValidation test",
		Contacts: []string{
			"camurcu@chromium.org",
			"arc-performance@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"android_p", "chrome"},
		ServiceDeps:  []string{"tast.cros.arc.CacheValidationService"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
			Val:               []string{"git_pi-arc-linux-apps"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
			Val:               []string{"git_pi-arcvm-dev-linux-apps", "--enable-arcvm"},
		}},
		Timeout: 5 * time.Minute,
	})
}

func CacheValidation(ctx context.Context, s *testing.State) {
	d := s.DUT()

	extraArgs := s.Param().([]string)

	args := &arcpb.Args{
		ExtraArgs: extraArgs,
	}

	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	service := arc.NewCacheValidationServiceClient(cl.Conn)

	// Generate gs:// URL for jar
	url, err := service.GetJarURL(ctx, args)
	if err != nil {
		s.Fatal("CacheValidationService.GetJarURL returned an error: ", err)
	}

	// Download jar from bucket
	tempJar, err := ioutil.TempFile("", filepath.Base(url.Url))
	if err != nil {
		s.Fatal(errors.Wrapf(err, "failed to create temp file for %s: ", filepath.Base(url.Url)))
	}

	defer os.Remove(tempJar.Name())

	if out, err := exec.Command("gsutil", "copy", url.Url, tempJar.Name()).CombinedOutput(); err != nil {
		s.Fatal(errors.Wrapf(err, "failed to upload ARC ureadahead pack to the server %q", out))
	}

	res, err := service.GetResult(ctx, args)
	if err != nil {
		s.Fatal("CacheValidationService.GetResult returned an error: ", err)
	}

	tpwc, err := ioutil.TempFile("", filepath.Base(res.PackagesWithCache))
	if err != nil {
		s.Fatal(errors.Wrapf(err, "failed to create temp file for %q", res.PackagesWithCache))
	}
	defer os.Remove(tpwc.Name())

	if err := d.GetFile(ctx, res.PackagesWithCache, tpwc.Name()); err != nil {
		s.Fatal(errors.Wrapf(err, "failed to get %q from the device", res.PackagesWithCache))
	}

	tpwoc, err := ioutil.TempFile("", filepath.Base(res.PackagesWithoutCache))
	if err != nil {
		s.Fatal(errors.Wrapf(err, "failed to create temp file for %q", res.PackagesWithoutCache))
	}
	defer os.Remove(tpwoc.Name())

	if err := d.GetFile(ctx, res.PackagesWithoutCache, tpwoc.Name()); err != nil {
		s.Fatal(errors.Wrapf(err, "failed to get %q from the device", res.PackagesWithoutCache))
	}

	tgpc, err := ioutil.TempFile("", filepath.Base(res.GeneratedPackagesCache))
	if err != nil {
		s.Fatal(errors.Wrapf(err, "failed to create temp file for %q", res.GeneratedPackagesCache))
	}
	defer os.Remove(tgpc.Name())

	if err := d.GetFile(ctx, res.GeneratedPackagesCache, tgpc.Name()); err != nil {
		s.Fatal(errors.Wrapf(err, "failed to get %q from the device", res.GeneratedPackagesCache))
	}

	javaClass := "org.chromium.arc.cachebuilder.Validator"

	if err := exec.Command("java", "-cp", tempJar.Name(), javaClass, "--source", tpwc.Name(), "--reference", tpwoc.Name(), "--dynamic-validate", "yes").Run(); err != nil {
		s.Fatal("validator failed: ", err)
	}

	if err := exec.Command("java", "-cp", tempJar.Name(), javaClass, "--source", tpwoc.Name(), "--reference", tgpc.Name(), "--dynamic-validate", "no").Run(); err != nil {
		s.Fatal("validator failed: ", err)
	}

	devtd := &arcpb.TempDir{
		TempDir: res.TempDir,
	}

	if _, err := service.RemoveTempFiles(ctx, devtd); err != nil {
		s.Fatal("CacheValidationService.RemoveTempFiles returned an error: ", err)
	}
}

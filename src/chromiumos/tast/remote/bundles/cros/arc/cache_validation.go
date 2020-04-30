// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/dut"
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

// generateJarURL gets ARC build properties from the device, parses for build ID, and
// generates gs URL for org.chromium.ard.cachebuilder.jar
func generateJarURL(ctx context.Context, dut *dut.DUT, propertyFile, branch string) (string, error) {
	const (
		// Base path
		buildsRoot = "gs://chromeos-arc-images/builds"

		// Name of jar file
		jarName = "org.chromium.arc.cachebuilder.jar"
	)

	buildProp, err := dut.Command("cat", propertyFile).Output(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to read ARC build property file remotely")
	}

	buildPropStr := string(buildProp)

	mBuildID := regexp.MustCompile(`(\n|^)ro.build.version.incremental=(.+)(\n|$)`).FindStringSubmatch(buildPropStr)
	if mBuildID == nil {
		return "", errors.Errorf("ro.build.version.incremental is not found in %q", buildPropStr)
	}

	url := fmt.Sprintf("%s/%s/%s/%s", buildsRoot, branch, mBuildID[2], jarName)
	return url, nil
}

func CacheValidation(ctx context.Context, s *testing.State) {
	d := s.DUT()

	extraArgs := s.Param().([]string)
	args := &arcpb.Args{
		ExtraArgs: extraArgs[1:],
	}

	var propertyFile string
	if len(extraArgs) > 1 && extraArgs[1] == "--enable-arcvm" {
		propertyFile = "/usr/share/arcvm/properties/build.prop"

	} else {
		propertyFile = "/usr/share/arc/properties/build.prop"
	}

	url, err := generateJarURL(ctx, d, propertyFile, extraArgs[0])
	if err != nil {
		s.Fatal("Failed to generate jar URL: ", err)
	}

	// Download jar from bucket
	tempJar, err := ioutil.TempFile("", filepath.Base(url))
	if err != nil {
		s.Fatal(errors.Wrapf(err, "failed to create temp file for %s: ", filepath.Base(url)))
	}

	defer os.Remove(tempJar.Name())

	if out, err := exec.Command("gsutil", "copy", url, tempJar.Name()).CombinedOutput(); err != nil {
		s.Fatal(errors.Wrapf(err, "failed to upload ARC ureadahead pack to the server %q", out))
	}

	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	service := arc.NewCacheValidationServiceClient(cl.Conn)

	res, err := service.Generate(ctx, args)
	if err != nil {
		s.Fatal("CacheValidationService.Generate returned an error: ", err)
	}

	// Gets file from DUT and puts it in a local temp dir. Returns temp path.
	getFile := func(file string) string {
		temp, err := ioutil.TempFile("", filepath.Base(file))
		if err != nil {
			s.Fatal(errors.Wrapf(err, "failed to create temp file for %q", file))
		}

		if err := d.GetFile(ctx, file, temp.Name()); err != nil {
			s.Fatal(errors.Wrapf(err, "failed to get %q from the device", file))
		}
		return temp.Name()
	}

	withCache := getFile(res.PackagesWithCache)
	defer os.Remove(withCache)
	withoutCache := getFile(res.PackagesWithoutCache)
	defer os.Remove(withoutCache)
	generated := getFile(res.GeneratedPackagesCache)
	defer os.Remove(generated)

	devtd := &arcpb.TempDir{
		TempDir: res.TempDir,
	}

	if _, err := service.RemoveTempFiles(ctx, devtd); err != nil {
		s.Fatal("CacheValidationService.RemoveTempFiles returned an error: ", err)
	}

	javaClass := "org.chromium.arc.cachebuilder.Validator"

	if err := exec.Command("java", "-cp", tempJar.Name(), javaClass, "--source", withCache, "--reference", withoutCache, "--dynamic-validate", "yes").Run(); err != nil {
		s.Fatal("Failed to validate withCache against withoutCache: ", err)
	}

	if err := exec.Command("java", "-cp", tempJar.Name(), javaClass, "--source", withoutCache, "--reference", generated, "--dynamic-validate", "no").Run(); err != nil {
		s.Fatal("Failed to validate withoutCache against generated: ", err)
	}
}

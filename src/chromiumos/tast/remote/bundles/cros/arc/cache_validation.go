// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CacheValidation,
		Desc: "Validates that caches match for both modes when pre-generated packages cache is enabled and disabled",
		Contacts: []string{
			"camurcu@chromium.org", // author
			"khmel@google.com",
			"arc-performance@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.arc.GmsCoreCacheService"},
		Params: []testing.Param{{
			Name:              "pi_container",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: []string{
				"/usr/share/arc/properties/build.prop",
				"git_pi-arc-linux-apps",
			},
		}, {
			Name:              "r",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: []string{
				"/usr/share/arcvm/properties/build.prop",
				"git_rvc-arc-dev-linux-apps",
			},
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

	params := s.Param().([]string)

	propertyFile := params[0]
	buildBranch := params[1]

	tempDir, err := ioutil.TempDir("", "tmp_dir")
	if err != nil {
		s.Fatal("Failed to create global temp dir: ", err)
	}

	defer os.RemoveAll(tempDir)

	url, err := generateJarURL(ctx, d, propertyFile, buildBranch)
	if err != nil {
		s.Fatal("Failed to generate jar URL: ", err)
	}

	jarPath := filepath.Join(tempDir, filepath.Base(url))

	if err := testexec.CommandContext(ctx, "gsutil", "copy", url, jarPath).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to download from %s: %v", url, err)
	}

	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	service := arc.NewGmsCoreCacheServiceClient(cl.Conn)

	// Makes the call to generate packages_cache.xml, gets the path, and returns
	// the local temp paths for both new and pregenerated packages caches and GMS core caches
	// after copying them over. Also returns the temp directory for removal.
	getCaches := func(cacheEnabled bool) (string, string, string, string) {
		request := arcpb.GmsCoreCacheRequest{
			PackagesCacheEnabled: cacheEnabled,
			GmsCoreEnabled:       cacheEnabled,
		}

		// Shorten the total context by 5 seconds to allow for cleanup.
		shortCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()

		// Call to generate packages_cache.xml
		response, err := service.Generate(shortCtx, &request)
		if err != nil {
			s.Fatal("GmsCoreCacheService.Generate returned an error: ", err)
		}
		defer d.Command("rm", "-rf", response.TargetDir).Output(ctx)

		newCacheFile := filepath.Join(response.TargetDir, response.PackagesCacheName)
		genCacheFile := filepath.Join(response.TargetDir, response.GeneratedPackagesCacheName)
		gmsCoreCacheTar := filepath.Join(response.TargetDir, response.GmsCoreCacheName)
		layoutFile := filepath.Join(response.TargetDir, "layout.txt")

		var subDir string
		if cacheEnabled {
			subDir = filepath.Join(tempDir, "withCache")
		} else {
			subDir = filepath.Join(tempDir, "withoutCache")
		}

		if err := os.Mkdir(subDir, os.ModePerm); err != nil {
			s.Fatal(errors.Wrap(err, "failed to created temp dir for GMS Core caches"))
		}

		// Gets file from DUT and returns local file path.
		getFile := func(file string) string {
			localFile := filepath.Join(subDir, filepath.Base(file))

			if err := d.GetFile(ctx, file, localFile); err != nil {
				s.Fatal(errors.Wrapf(err, "failed to get %q from the device", file))
			}
			return localFile
		}

		newCache := getFile(newCacheFile)
		genCache := getFile(genCacheFile)
		gmsCoreCache := getFile(gmsCoreCacheTar)
		layout := getFile(layoutFile)

		// Unpack GMS core caches
		if err = testexec.CommandContext(
			ctx, "tar", "-xvpf", gmsCoreCache, "-C", subDir).Run(); err != nil {
			s.Fatal(errors.Wrapf(err, "decompression %q failed", gmsCoreCache))
		}

		return newCache, genCache, layout, subDir
	}

	withCache, genCache, withCacheLayout, withCacheDir := getCaches(true)
	withoutCache, genCache, withoutCacheLayout, withoutCacheDir := getCaches(false)

	// saveOutput runs the command specified by name with args as arguments, and saves
	// the stdout and stderr to outPath.
	saveOutput := func(outPath string, cmd *testexec.Cmd) error {
		f, err := os.Create(outPath)
		if err != nil {
			return err
		}
		defer f.Close()
		cmd.Stdout = f
		cmd.Stderr = f
		return cmd.Run(testexec.DumpLogOnError)
	}

	s.Log("Validating GMS Core cache")
	// Note, vdex and odex are not guarented to be the same even if produced from the same sources.
	if err := saveOutput(filepath.Join(s.OutDir(), "app_chimera.diff"),
		testexec.CommandContext(ctx, "diff", "--recursive", "--no-dereference",
			"--exclude=*.odex", "--exclude=*.vdex",
			filepath.Join(withCacheDir, "app_chimera"),
			filepath.Join(withoutCacheDir, "app_chimera"))); err != nil {
		s.Error("Error validating app_chimera folders: ", err)
	}
	if err := saveOutput(filepath.Join(s.OutDir(), "layout.diff"),
		testexec.CommandContext(ctx, "diff", withCacheLayout, withoutCacheLayout)); err != nil {
		s.Error("Error validating app_chimera layouts: ", err)
	}

	const javaClass = "org.chromium.arc.cachebuilder.Validator"

	s.Log("Validating Packages cache")
	if err := testexec.CommandContext(
		ctx, "java", "-cp", jarPath, javaClass,
		"--source", withCache,
		"--reference", withoutCache,
		"--dynamic-validate", "yes").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to validate withCache against withoutCache: ", err)
	}

	if err := testexec.CommandContext(
		ctx, "java", "-cp", jarPath, javaClass,
		"--source", withoutCache,
		"--reference", genCache,
		"--dynamic-validate", "no").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Failed to validate withoutCache against generated: ", err)
	}
}

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

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DataCollector,
		Desc: "Signs in to DUT and performs ARC++ boot with various paramters. Captures required data and uploads it to Chrome binary server. This data is used by various tools. Normally, this test should be run during the Android PFQ, once per build/arch",
		Contacts: []string{
			"khmel@chromium.org", // Original author.
			"arc-performance@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		// TODO(b/150012956): Stop using 'arc' here and use ExtraSoftwareDeps instead.
		SoftwareDeps: []string{"arc", "chrome"},
		ServiceDeps: []string{"tast.cros.arc.UreadaheadPackService",
			"tast.cros.arc.GmsCoreCacheService"},
		Timeout: 10 * time.Minute,
		Vars: []string{
			"arc.UreadaheadService.username",
			"arc.UreadaheadService.password",
		},
	})
}

// getArcVersionRemotely gets ARC build properties from the device, parses for build ID, ABI, and
// returns these fields as a combined string. It also return weither this is official build or not
func getArcVersionRemotely(ctx context.Context, dut *dut.DUT) (bool, string, error) {
	isARCVM, err := dut.Command("cat", "/run/chrome/is_arcvm").Output(ctx)
	if err != nil {
		return false, "", errors.Wrap(err, "failed to check ARCVM status remotely")
	}

	var propertyFile string
	if string(isARCVM) == "1" {
		propertyFile = "/usr/share/arcvm/properties/build.prop"
	} else {
		propertyFile = "/usr/share/arc/properties/build.prop"
	}

	buildProp, err := dut.Command("cat", propertyFile).Output(ctx)
	if err != nil {
		return false, "", errors.Wrap(err, "failed to read ARC build property file remotely")
	}
	buildPropStr := string(buildProp)

	mArch := regexp.MustCompile(`(\n|^)ro.product.cpu.abi=(.+)(\n|$)`).FindStringSubmatch(buildPropStr)
	if mArch == nil {
		return false, "", errors.Errorf("ro.product.cpu.abi is not found in %q", buildPropStr)
	}

	// Note, this should work on official builds only. Custom built Android image contains the
	// version in different format.
	mVersion := regexp.MustCompile(`(\n|^)ro.build.version.incremental=(.+)(\n|$)`).FindStringSubmatch(buildPropStr)
	if mVersion == nil {
		return false, "", errors.Errorf("ro.build.version.incremental is not found in %q", buildPropStr)
	}

	official := regexp.MustCompile(`^\d+$`).MatchString(mVersion[2])
	result := fmt.Sprintf("%s_%s", mArch[2], mVersion[2])
	return official, result, nil
}

// DataCollector performs ARC++ boots in various conditions, grabs required data and uploads it to
// the binary server.
func DataCollector(ctx context.Context, s *testing.State) {
	const (
		// Base path for uploaded resources.
		runtimeArtefactsRoot = "gs://chromeos-arc-images/runtime_artefacts"

		// ureadahed packs bucket
		ureadAheadPacks = "ureadahead_packs"

		// GMS Core caches bucket
		gmsCoreCaches = "gms_core_caches"

		// Name of the pack in case of initial boot.
		initialPack = "initial_pack"

		// Name of the pack in case of provisioned boot.
		provisionedPack = "provisioned_pack"
	)

	d := s.DUT()

	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	remoteURL := func(bucket, version, dst string) string {
		return fmt.Sprintf("%s/%s/%s/%s", runtimeArtefactsRoot, bucket, version, dst)
	}

	// upload gets file from the target device and uploads it to the server.
	upload := func(ctx context.Context, dut *dut.DUT, src, bucket, version, dst string) error {
		temp, err := ioutil.TempFile("", filepath.Base(src))
		if err != nil {
			return errors.Wrapf(err, "failed to create temp file for %q", src)
		}
		defer os.Remove(temp.Name())

		if err := dut.GetFile(ctx, src, temp.Name()); err != nil {
			return errors.Wrapf(err, "failed to get %q from the device", src)
		}

		gsURL := remoteURL(bucket, version, dst)

		// Use gsutil command to upload the file to the server.
		testing.ContextLogf(ctx, "Uploading %q to the server", gsURL)
		if out, err := exec.Command("gsutil", "copy", temp.Name(), gsURL).CombinedOutput(); err != nil {
			return errors.Wrapf(err, "failed to upload ARC ureadahead pack to the server %q", out)
		}

		testing.ContextLogf(ctx, "Uploaded %q to the server", gsURL)
		return nil
	}

	genUreadaheadPack := func() {
		official, v, err := getArcVersionRemotely(ctx, d)
		if err != nil {
			s.Fatal("Failed to get ARC version: ", err)
		}
		s.Logf("Detected version: %s", v)

		service := arc.NewUreadaheadPackServiceClient(cl.Conn)
		// First boot is needed to be initial boot with removing all user data.
		request := arcpb.UreadaheadPackRequest{
			InitialBoot: true,
			Username:    s.RequiredVar("arc.UreadaheadService.username"),
			Password:    s.RequiredVar("arc.UreadaheadService.password"),
		}

		// Checks if generated packs need to be uploaded to the server.
		needUpload := func() bool {
			if !official {
				s.Logf("Version: %s is not official version and generated ureadahead packs won't be uploaded to the server", v)
				return false
			}

			packs := []string{initialPack, provisionedPack}
			for _, pack := range packs {
				gsURL := remoteURL(ureadAheadPacks, v, pack)
				if err := exec.Command("gsutil", "stat", gsURL).Run(); err != nil {
					return true
				}
			}

			s.Logf("Version: %s has all packs uploaded and generated ureadahead packs won't be uploaded to the server", v)
			return false
		}

		needUploadPacks := needUpload()

		// Shorten the total context by 5 seconds to allow for cleanup.
		shortCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()

		// Due to race condition of using ureadahead in various parts of Chrome,
		// first generation might be incomplete. Just pass it without analyzing.
		if _, err := service.Generate(shortCtx, &request); err != nil {
			s.Fatal("UreadaheadPackService.Generate returned an error for warm-up pass: ", err)
		}

		// Pass initial boot and capture results.
		response, err := service.Generate(shortCtx, &request)
		if err != nil {
			s.Fatal("UreadaheadPackService.Generate returned an error for initial boot pass: ", err)
		}

		if needUploadPacks {
			if err = upload(shortCtx, d, response.PackPath, ureadAheadPacks, v, initialPack); err != nil {
				s.Fatal("Failed to upload initial boot pack: ", err)
			}
		}

		// Now pass provisioned boot and capture results.
		request.InitialBoot = false
		response, err = service.Generate(shortCtx, &request)
		if err != nil {
			s.Fatal("UreadaheadPackService.Generate returned an error for second boot pass: ", err)
		}

		if needUploadPacks {
			if err = upload(shortCtx, d, response.PackPath, ureadAheadPacks, v, provisionedPack); err != nil {
				s.Fatal("Failed to upload provisioned boot pack: ", err)
			}
		}
	}
	genUreadaheadPack()

	genGmsCoreCache := func() {
		service := arc.NewGmsCoreCacheServiceClient(cl.Conn)

		// Shorten the total context by 5 seconds to allow for cleanup.
		shortCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()

		response, err := service.Generate(shortCtx, &empty.Empty{})
		if err != nil {
			s.Fatal("GmsCoreCacheService.Generate returned an error: ", err)
		}
		defer d.Command("rm", "-rf", response.TargetDir).Output(ctx)

		packages, err := d.Command("cat", filepath.Join(response.TargetDir, response.PackagesCacheName)).Output(ctx)
		if err != nil {
			s.Fatal("Failed to read packages_cache.xml: ", err)
		}

		gmsCoreVersion := regexp.MustCompile(`<package name=\"com\.google\.android\.gms\".+primaryCpuAbi=\"(\S+)\".+version=\"(\d+)\".+>`).FindStringSubmatch(string(packages))
		if gmsCoreVersion == nil {
			s.Fatal("Failed to parse GMS Core version from packages_cache.xml")
		}

		v := fmt.Sprintf("%s_%s", gmsCoreVersion[1], gmsCoreVersion[2])
		s.Logf("Detected GMS core version: %s", v)

		// Checks if generated GMS Core caches need to be uploaded to the server.
		resources := []string{response.GmsCoreCacheName, response.GmsCoreManifestName, response.GsfCacheName}
		needUpload := func() bool {
			for _, resource := range resources {
				gsURL := remoteURL(gmsCoreCaches, v, resource)
				if err := exec.Command("gsutil", "stat", gsURL).Run(); err != nil {
					return true
				}
			}

			s.Logf("GMS Core: %s has all resources uploaded and generated caches won't be uploaded to the server", v)
			return false
		}

		if needUpload() {
			for _, resource := range resources {
				if err = upload(
					shortCtx,
					d,
					filepath.Join(response.TargetDir, resource),
					gmsCoreCaches,
					v,
					resource); err != nil {
					s.Fatalf("Failed to upload %q: %v", resource, err)
				}
			}
		}
	}
	genGmsCoreCache()
}

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

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

type buildDescriptor struct {
	// true in case built by ab/
	official bool
	// ab/buildID
	buildID string
	// build type e.g. user, userdebug
	buildType string
	// cpu abi e.g. x86_64, x86, arm
	cpuAbi string
}

type testParam struct {
	vmEnabled bool
	// if set, collected data will be upload to cloud.
	upload bool
	// if set, keep local data in this directory.
	dataDir string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: DataCollector,
		Desc: "Signs in to DUT and performs ARC++ boot with various paramters. Captures required data and uploads it to Chrome binary server. This data is used by various tools. Normally, this test should be run during the Android PFQ, once per build/arch",
		Contacts: []string{
			"khmel@chromium.org", // Original author.
			"arc-performance@google.com",
		},
		Attr: []string{"group:mainline"},
		// TODO(b/150012956): Stop using 'arc' here and use ExtraSoftwareDeps instead.
		SoftwareDeps: []string{"arc", "chrome"},
		ServiceDeps: []string{"tast.cros.arc.UreadaheadPackService",
			"tast.cros.arc.GmsCoreCacheService"},
		Timeout: 20 * time.Minute,
		// Note that arc.DataCollector is not a simple test. It collects data used to
		// produce test and release images. Not collecting this data leads to performance
		// regression and failure of other tests. Please consider fixing the issue rather
		// then disabling this in Android PFQ. At this time missing the data is allowed
		// for the grace perioid however it will be a build stopper after.
		Params: []testing.Param{{
			ExtraAttr:         []string{"group:mainline"},
			ExtraSoftwareDeps: []string{"android_p"},
			Val: testParam{
				vmEnabled: false,
				upload:    true,
				dataDir:   "",
			},
		}, {
			Name:              "vm",
			ExtraAttr:         []string{"group:arc-data-collector"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: testParam{
				vmEnabled: true,
				upload:    true,
				dataDir:   "",
			},
		}, {
			Name:              "local",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: testParam{
				vmEnabled: false,
				upload:    false,
				dataDir:   "/tmp/data_collector",
			},
		}, {
			Name:              "vm_local",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: testParam{
				vmEnabled: true,
				upload:    false,
				dataDir:   "/tmp/data_collector",
			},
		}},
		Vars: []string{
			"arc.DataCollector.UreadaheadService_username",
			"arc.DataCollector.UreadaheadService_password",
		},
	})
}

// getBuildDescriptorRemotely gets ARC build properties from the device, parses for build ID, ABI,
// and returns these fields as a combined string. It also return weither this is official build or
// not.
func getBuildDescriptorRemotely(ctx context.Context, dut *dut.DUT, vmEnabled bool) (*buildDescriptor, error) {
	var propertyFile string
	if vmEnabled {
		propertyFile = "/usr/share/arcvm/properties/build.prop"
	} else {
		propertyFile = "/usr/share/arc/properties/build.prop"
	}

	buildProp, err := dut.Command("cat", propertyFile).Output(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read ARC build property file remotely")
	}
	buildPropStr := string(buildProp)

	mCPUAbi := regexp.MustCompile(`(\n|^)ro.product.cpu.abi=(.+)(\n|$)`).FindStringSubmatch(buildPropStr)
	if mCPUAbi == nil {
		return nil, errors.Errorf("ro.product.cpu.abi is not found in %q", buildPropStr)
	}

	mBuildType := regexp.MustCompile(`(\n|^)ro.build.type=(.+)(\n|$)`).FindStringSubmatch(buildPropStr)
	if mBuildType == nil {
		return nil, errors.Errorf("ro.product.cpu.abi is not found in %q", buildPropStr)
	}

	// Note, this should work on official builds only. Custom built Android image contains the
	// version in different format.
	mBuildID := regexp.MustCompile(`(\n|^)ro.build.version.incremental=(.+)(\n|$)`).FindStringSubmatch(buildPropStr)
	if mBuildID == nil {
		return nil, errors.Errorf("ro.build.version.incremental is not found in %q", buildPropStr)
	}

	abiMap := map[string]string{
		"armeabi-v7a": "arm",
		"arm64-v8a":   "arm64",
		"x86":         "x86",
		"x86_64":      "x86_64",
	}

	abi, ok := abiMap[mCPUAbi[2]]
	if !ok {
		return nil, errors.Errorf("failed to map ABI %q", mCPUAbi[2])
	}

	desc := buildDescriptor{
		official:  regexp.MustCompile(`^\d+$`).MatchString(mBuildID[2]),
		buildID:   mBuildID[2],
		buildType: mBuildType[2],
		cpuAbi:    abi,
	}

	return &desc, nil
}

// DataCollector performs ARC++ boots in various conditions, grabs required data and uploads it to
// the binary server.
func DataCollector(ctx context.Context, s *testing.State) {
	const (
		// Base path for uploaded resources.
		runtimeArtifactsRoot = "gs://chromeos-arc-images/runtime_artifacts"

		// ureadahed packs bucket
		ureadAheadPack = "ureadahead_pack"

		// GMS Core caches bucket
		gmsCoreCache = "gms_core_cache"

		// Name of the pack in case of initial boot.
		initialPack = "initial_pack"

		// Name of the pack in case of provisioned boot.
		provisionedPack = "provisioned_pack"

		// Name of gsutil
		gsUtil = "gsutil"

		// Number of retries for each flow in case of failure.
		// TODO(b/167697547): Set retryCount to 0 once bug gets fixed.
		retryCount = 2
	)

	d := s.DUT()

	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	remoteURL := func(bucket, version string) string {
		return fmt.Sprintf("%s/%s_%s.tar", runtimeArtifactsRoot, bucket, version)
	}

	param := s.Param().(testParam)

	desc, err := getBuildDescriptorRemotely(ctx, d, param.vmEnabled)
	if err != nil {
		s.Fatal("Failed to get ARC build desc: ", err)
	}

	v := fmt.Sprintf("%s_%s_%s", desc.cpuAbi, desc.buildType, desc.buildID)
	s.Logf("Detected version: %s", v)

	// Checks if generated resources need to be uploaded to the server.
	needUpload := func(bucket string) bool {
		if !param.upload {
			s.Log("Cloud upload is disabled")
			return false
		}
		if !desc.official {
			s.Logf("Version: %s is not official version and generated ureadahead packs won't be uploaded to the server", v)
			return false
		}

		gsURL := remoteURL(bucket, v)
		if err := exec.Command(gsUtil, "stat", gsURL).Run(); err != nil {
			return true
		}

		// This test is scheduled to run once per build id and ARCH. So race should never happen.
		s.Logf("%q exists and won't be uploaded to the server", gsURL)
		return false
	}

	upload := func(ctx context.Context, src, bucket string) error {
		gsURL := remoteURL(bucket, v)

		// Use gsutil command to upload the file to the server.
		testing.ContextLogf(ctx, "Uploading %q to the server", gsURL)
		if out, err := exec.Command(gsUtil, "copy", src, gsURL).CombinedOutput(); err != nil {
			return errors.Wrapf(err, "failed to upload ARC ureadahead pack to the server %q", out)
		}

		testing.ContextLogf(ctx, "Uploaded %q to the server", gsURL)

		// AllUsers read access is considered safe for two reasons.
		// First, this data is included into the image unmodified.
		// Second, we already practice setting this permission for other Android build
		// artifacts. For example from APPS bucket.
		if out, err := exec.Command(gsUtil, "acl", "ch", "-u", "AllUsers:READ", gsURL).CombinedOutput(); err != nil {
			return errors.Wrapf(err, "failed to upload ARC ureadahead pack to the server %q", out)
		}

		testing.ContextLogf(ctx, "Set read permission for  %q", gsURL)

		return nil
	}

	dataDir := param.dataDir
	// If data dir is not provided, use temp folder and remove after use.
	if dataDir == "" {
		dataDir, err = ioutil.TempDir("", "data_collector")
		if err != nil {
			s.Fatal("Failed to create temp dir: ", err)
		}
		os.Chmod(dataDir, 0744)
		defer os.RemoveAll(dataDir)
	} else {
		// Clean up before use.
		os.RemoveAll(dataDir)
		err := os.Mkdir(dataDir, 0744)
		if err != nil {
			s.Fatal("Failed to create local dir: ", err)
		}
	}

	genUreadaheadPack := func() error {
		service := arc.NewUreadaheadPackServiceClient(cl.Conn)
		// First boot is needed to be initial boot with removing all user data.
		request := arcpb.UreadaheadPackRequest{
			InitialBoot: true,
			Username:    s.RequiredVar("arc.DataCollector.UreadaheadService_username"),
			Password:    s.RequiredVar("arc.DataCollector.UreadaheadService_password"),
			VmEnabled:   param.vmEnabled,
		}

		// Shorten the total context by 5 seconds to allow for cleanup.
		shortCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()

		// Due to race condition of using ureadahead in various parts of Chrome,
		// first generation might be incomplete. Just pass it without analyzing.
		if _, err := service.Generate(shortCtx, &request); err != nil {
			return errors.Wrap(err, "ureadaheadPackService.Generate returned an error for warm-up pass")
		}

		// Pass initial boot and capture results.
		response, err := service.Generate(shortCtx, &request)
		if err != nil {
			return errors.Wrap(err, "ureadaheadPackService.Generate returned an error for initial boot pass")
		}

		targetDir := filepath.Join(dataDir, ureadAheadPack)
		if err = os.Mkdir(targetDir, 0744); err != nil {
			s.Fatalf("Failed to create %q: %v", targetDir, err)
		}

		if err = d.GetFile(shortCtx, response.PackPath, filepath.Join(targetDir, initialPack)); err != nil {
			s.Fatalf("Failed to get %q from the device: %v", response.PackPath, err)
		}

		// Now pass provisioned boot and capture results.
		request.InitialBoot = false
		response, err = service.Generate(shortCtx, &request)
		if err != nil {
			return errors.Wrap(err, "ureadaheadPackService.Generate returned an error for second boot pass: ")
		}

		if err = d.GetFile(shortCtx, response.PackPath, filepath.Join(targetDir, provisionedPack)); err != nil {
			s.Fatalf("Failed to get %q from the device: %v", response.PackPath, err)
		}

		targetTar := filepath.Join(targetDir, v+".tar")
		testing.ContextLogf(shortCtx, "Compressing ureadahead packs to %q", targetTar)
		if err = exec.Command("tar", "-cvpf", targetTar, "-C", targetDir, ".").Run(); err != nil {
			s.Fatalf("Failed to compress %q: %v", targetDir, err)
		}

		if needUpload(ureadAheadPack) {
			if err := upload(shortCtx, targetTar, ureadAheadPack); err != nil {
				s.Fatalf("Failed to upload %q: %v", ureadAheadPack, err)
			}

		}

		return nil
	}
	attempts := 0
	for {
		err := genUreadaheadPack()
		if err == nil {
			break
		}
		attempts = attempts + 1
		if attempts > retryCount {
			s.Fatal("Failed to generate ureadahead packs. No more retries left: ", err)
		}
		s.Log("Retrying generating ureadahead, previous attempt failed: ", err)
	}

	genGmsCoreCache := func() error {
		service := arc.NewGmsCoreCacheServiceClient(cl.Conn)

		request := arcpb.GmsCoreCacheRequest{
			PackagesCacheEnabled: true,
			GmsCoreEnabled:       false,
		}

		// Shorten the total context by 5 seconds to allow for cleanup.
		shortCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()

		response, err := service.Generate(shortCtx, &request)
		if err != nil {
			return errors.Wrap(err, "failed to generate GMS Core caches")
		}
		defer d.Command("rm", "-rf", response.TargetDir).Output(ctx)

		targetDir := filepath.Join(dataDir, gmsCoreCache)
		if err = os.Mkdir(targetDir, 0744); err != nil {
			s.Fatalf("Failed to create %q: %v", targetDir, err)
		}

		resources := []string{response.GmsCoreCacheName, response.GmsCoreManifestName, response.GsfCacheName}
		for _, resource := range resources {
			if err = d.GetFile(shortCtx, filepath.Join(response.TargetDir, resource), filepath.Join(targetDir, resource)); err != nil {
				s.Fatalf("Failed to get %q from the device: %v", resource, err)
			}
		}
		targetTar := filepath.Join(targetDir, v+".tar")
		testing.ContextLogf(shortCtx, "Compressing gms core caches to %q", targetTar)
		if err = exec.Command("tar", "-cvpf", targetTar, "-C", targetDir, ".").Run(); err != nil {
			s.Fatalf("Failed to compress %q: %v", targetDir, err)
		}

		if needUpload(gmsCoreCache) {
			if err := upload(shortCtx, targetTar, gmsCoreCache); err != nil {
				s.Fatalf("Failed to upload %q: %v", gmsCoreCache, err)
			}
		}

		return nil
	}
	attempts = 0
	for {
		err := genGmsCoreCache()
		if err == nil {
			break
		}
		attempts = attempts + 1
		if attempts > retryCount {
			s.Fatal("Failed to generate GMS Core caches. No more retries left: ", err)
		}
		s.Log("Retrying generating GMS Core caches, previous attempt failed: ", err)
	}

}

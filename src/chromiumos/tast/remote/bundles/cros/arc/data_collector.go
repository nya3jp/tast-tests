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
	"chromiumos/tast/fsutil"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/ssh/linuxssh"
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
			"alanding@chromium.org",
			"arc-performance@google.com",
		},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		ServiceDeps: []string{"tast.cros.arc.UreadaheadPackService",
			"tast.cros.arc.GmsCoreCacheService"},
		Timeout: 20 * time.Minute,
		// Note that arc.DataCollector is not a simple test. It collects data used to
		// produce test and release images. Not collecting this data leads to performance
		// regression and failure of other tests. Please consider fixing the issue rather
		// then disabling this in Android PFQ. At this time missing the data is allowed
		// for the grace perioid however it will be a build stopper after.
		Params: []testing.Param{{
			ExtraAttr:         []string{"group:arc-data-collector"},
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
		Vars: []string{"ui.gaiaPoolDefault"},
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
		// Please see b/167697547, b/181832600 for more information. Retries are
		// needed for occasional OptIn instability on ARC development builds. Only
		// lower count if for sure OptIn is completely stable.
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

	genUreadaheadPack := func() (retErr error) {
		service := arc.NewUreadaheadPackServiceClient(cl.Conn)
		// First boot is needed to be initial boot with removing all user data.
		request := arcpb.UreadaheadPackRequest{
			InitialBoot: true,
			Creds:       s.RequiredVar("ui.gaiaPoolDefault"),
			VmEnabled:   param.vmEnabled,
		}

		// Shorten the total context by 5 seconds to allow for cleanup.
		shortCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()

		// Pass initial boot and capture results.
		response, err := service.Generate(shortCtx, &request)
		if err != nil {
			return errors.Wrap(err, "ureadaheadPackService.Generate returned an error for initial boot pass")
		}

		targetDir := filepath.Join(dataDir, ureadAheadPack)
		defer func() {
			// Cleanup in case of failure. Might be needed for next retry passes.
			if retErr != nil {
				os.RemoveAll(targetDir)
			}
		}()
		if err = os.Mkdir(targetDir, 0744); err != nil {
			s.Fatalf("Failed to create %q: %v", targetDir, err)
		}

		intitalPackPath := filepath.Join(targetDir, initialPack)
		provisionedPackPath := filepath.Join(targetDir, provisionedPack)
		if err = linuxssh.GetFile(shortCtx, d.Conn(), response.PackPath, intitalPackPath); err != nil {
			s.Fatalf("Failed to get %q from the device: %v", response.PackPath, err)
		}

		// Initial implementation of DataCollector had here second pass that booted over
		// the already provisioned account. However nowadays this pack is no longer used
		// but is still referenced from the ChromeOS build system. Recently we switched to
		// the pool of accounts and this introduces an inconsistency when the already
		// provisioned pass exececuted with a random account, which in most cases does not
		// match one, used for initial provisioning.  As a temporary solution, copy the
		// initial pack to provisioned pack in order to satisfy ebuild requirements.
		// TODO(b/182294127): Discard the reference to the provisioned pack in build system
		// and remove this coping.
		if err := fsutil.CopyFile(intitalPackPath, provisionedPackPath); err != nil {
			s.Fatal("Failed copying file: ", err)
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
			if err = linuxssh.GetFile(shortCtx, d.Conn(), filepath.Join(response.TargetDir, resource), filepath.Join(targetDir, resource)); err != nil {
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

	// Helper that dumps logcat on failure. Dumping is optional and error here does not break
	// DataCollector flow and retries on error.
	dumpLogcat := func(mode string, attempt int) {
		log, err := d.Conn().Command(
			"/usr/sbin/android-sh",
			"-c",
			"/system/bin/logcat -d").Output(ctx)
		if err != nil {
			s.Logf("Failed to dump logcat, continue after this error: %q", err)
			return
		}

		logcatPath := filepath.Join(s.OutDir(), fmt.Sprintf("logcat_%s_%d.txt", mode, attempt))
		err = ioutil.WriteFile(logcatPath, log, 0644)
		if err != nil {
			s.Logf("Failed to save logcat, continue after this error: %q", err)
			return
		}
		s.Logf("Logcat for failure was saved to : %q", logcatPath)
	}

	attempts := 0
	for {
		err := genGmsCoreCache()
		if err == nil {
			break
		}
		attempts = attempts + 1
		dumpLogcat("gms_core", attempts)
		if attempts > retryCount {
			s.Fatal("Failed to generate GMS Core caches. No more retries left: ", err)
		}
		s.Log("Retrying generating GMS Core caches, previous attempt failed: ", err)
	}

	// Due to race condition of using ureadahead in various parts of Chrome,
	// first generation might be incomplete. Pass GMS Core cache generation as a warm-up
	// for ureadahead generation.
	attempts = 0
	for {
		err := genUreadaheadPack()
		if err == nil {
			break
		}
		attempts = attempts + 1
		dumpLogcat("ureadahead", attempts)
		if attempts > retryCount {
			s.Fatal("Failed to generate ureadahead packs. No more retries left: ", err)
		}
		s.Log("Retrying generating ureadahead, previous attempt failed: ", err)
	}
}

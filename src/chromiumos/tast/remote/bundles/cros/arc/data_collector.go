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
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/arc/version"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

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
		Timeout: 40 * time.Minute,
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
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

// getMemoryTotalKB returns total memory available in kilobytes for DUT.
func getMemoryTotalKB(ctx context.Context, dut *dut.DUT) (int, error) {
	memInfo, err := dut.Command("cat", "/proc/meminfo").Output(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to read /proc/meminfo")
	}

	memTotal := regexp.MustCompile(`(\n|^)MemTotal:\s+(\d+)\s+kB(\n|$)`).FindSubmatch(memInfo)
	if memTotal == nil {
		return 0, errors.Errorf("required MemTotal is not found in %q", memInfo)
	}
	memTotalInt, err := strconv.Atoi(string(memTotal[2]))
	if err != nil || memTotalInt <= 0 {
		return 0, errors.Errorf("failed to parse %q", memTotal[2])
	}

	return memTotalInt, nil
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

		// Name of the log for pack in case of initial boot.
		initialPackLog = "initial_pack.log"

		// Name of the pack in case of initial boot inside VM.
		// TODO(b/183648019): Once VM ureadahead flow is stable, remove this and
		// change vm_initial_pack -> initial_pack.
		vmInitialPack = "vm_initial_pack"

		// Name of gsutil
		gsUtil = "gsutil"

		// Number of retries for each flow in case of failure.
		// Please see b/167697547, b/181832600 for more information. Retries are
		// needed for occasional OptIn instability on ARC development builds. Only
		// lower count if for sure OptIn is completely stable.
		retryCount = 2

		// It is known issue that 4G devices experience memory pressure during the opt in.
		// This leads to the situation when FS page caches are reclaimed and captured result
		// does not properly relfect actual FS usage. Don't upload caches to server for
		// devices lower than 8G.
		minVMMemoryKB = 7500000
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

	desc, err := version.GetBuildDescriptorRemotely(ctx, d, param.vmEnabled)
	if err != nil {
		s.Fatal("Failed to get ARC build desc: ", err)
	}

	v := fmt.Sprintf("%s_%s_%s", desc.CPUAbi, desc.BuildType, desc.BuildID)
	s.Logf("Detected version: %s", v)

	// Checks if generated resources need to be uploaded to the server.
	needUpload := func(bucket string) bool {
		if !param.upload {
			s.Log("Cloud upload is disabled")
			return false
		}

		if !desc.Official {
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
			Creds: s.RequiredVar("ui.gaiaPoolDefault"),
		}

		// Shorten the total context by 5 seconds to allow for cleanup.
		shortCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()

		// Pass initial boot and capture results.
		response, err := service.Generate(shortCtx, &request)
		if err != nil {
			return errors.Wrap(err, "ureadaheadPackService.Generate returned an error for initial boot pass")
		}

		// Prepare target directory on host for pack file.
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

		var filesToGet = map[string]string{}
		filesToGet[response.PackPath] = initialPack
		filesToGet[response.LogPath] = initialPackLog

		if param.vmEnabled {
			filesToGet[response.VmPackPath] = vmInitialPack
		}

		for source, targetShort := range filesToGet {
			target := filepath.Join(targetDir, targetShort)
			if err = linuxssh.GetFile(shortCtx, d.Conn(), source, target, linuxssh.PreserveSymlinks); err != nil {
				s.Fatalf("Failed to get %q from the device: %v", source, err)
			}
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
			if err = linuxssh.GetFile(shortCtx, d.Conn(), filepath.Join(response.TargetDir, resource), filepath.Join(targetDir, resource), linuxssh.PreserveSymlinks); err != nil {
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

	memTotalKB := 0
	if param.vmEnabled {
		if memTotalKB, err = getMemoryTotalKB(ctx, d); err != nil {
			s.Fatal("Failed to get memory info: ", err)
		}
		s.Logf("Detected memory total: %d kb", memTotalKB)
	}

	// Limit running in PFQ for VM devices to 8GB+ RAM spec only. For local test
	// configs (upload=false) and non-VM, there are no restrictions.
	if !param.vmEnabled || !param.upload || memTotalKB > minVMMemoryKB {
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
	} else {
		s.Logf("Device total memory %d does not meet %d required to run ureadahead on VM, skipping pack generation", memTotalKB, minVMMemoryKB)
	}
}

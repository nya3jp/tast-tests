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
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/ctxutil"
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
	// if set, TTS cache generation will run.
	ttsCacheEnabled bool
}

const (
	// Name of gsutil
	gsUtil = "gsutil"

	// Base path for uploaded resources.
	runtimeArtifactsRoot = "gs://chromeos-arc-images/runtime_artifacts"

	// TTS cache bucket
	ttsCache = "tts_cache"
)

type dataUploader struct {
	ctx             context.Context
	androidVersion  string
	shouldUpload    bool
	buildDescriptor *version.BuildDescriptor
}

func (du *dataUploader) needUpload(bucket string) bool {
	if !du.shouldUpload {
		testing.ContextLog(du.ctx, "Cloud upload is disabled")
		return false
	}

	if !du.buildDescriptor.Official {
		testing.ContextLogf(du.ctx, "Version: %s is not official version and generated ureadahead packs won't be uploaded to the server", du.androidVersion)
		return false
	}

	gsURL := du.remoteURL(bucket, du.androidVersion)
	// gsutil stat would return 1 for a non-existent object.
	if err := exec.Command(gsUtil, "stat", gsURL).Run(); err != nil {
		return true
	}

	// This test is scheduled to run once per build id and ARCH. So race should never happen.
	testing.ContextLogf(du.ctx, "%q exists and won't be uploaded to the server", gsURL)
	return false
}

func (du *dataUploader) remoteURL(bucket, version string) string {
	return fmt.Sprintf("%s/%s_%s.tar", runtimeArtifactsRoot, bucket, version)
}

func (du *dataUploader) upload(src, bucket string) error {
	gsURL := du.remoteURL(bucket, du.androidVersion)

	// Use gsutil command to upload the file to the server.
	testing.ContextLogf(du.ctx, "Uploading %q to the server", gsURL)
	if out, err := exec.Command(gsUtil, "copy", src, gsURL).CombinedOutput(); err != nil {
		return errors.Wrapf(err, "failed to upload %q to the server %q", src, out)
	}

	testing.ContextLogf(du.ctx, "Set read permission for  %q", gsURL)

	// AllUsers read access is considered safe for two reasons.
	// First, this data is included into the image unmodified.
	// Second, we already practice setting this permission for other Android build
	// artifacts. For example from APPS bucket.
	if out, err := exec.Command(gsUtil, "acl", "ch", "-u", "AllUsers:READ", gsURL).CombinedOutput(); err != nil {
		return errors.Wrapf(err, "failed to set read permission for %q to the server %q", gsURL, out)
	}

	return nil
}

func (du *dataUploader) uploadIfNeeded(src, bucket string) error {
	if du.needUpload(bucket) {
		return du.upload(src, bucket)
	}
	return nil
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DataCollector,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Signs in to DUT and performs ARC++ boot with various paramters. Captures required data and uploads it to Chrome binary server. This data is used by various tools. Normally, this test should be run during the Android PFQ, once per build/arch",
		Contacts: []string{
			"khmel@chromium.org", // Original author.
			"alanding@chromium.org",
			"arc-performance@google.com",
		},
		SoftwareDeps: []string{"arc_android_data_cros_access", "chrome", "chrome_internal"},
		ServiceDeps: []string{"tast.cros.arc.UreadaheadPackService",
			"tast.cros.arc.GmsCoreCacheService", "tast.cros.arc.TTSCacheService"},
		Timeout: 40 * time.Minute,
		// Note that arc.DataCollector is not a simple test. It collects data used to
		// produce test and release images. Not collecting this data leads to performance
		// regression and failure of other tests. Please consider fixing the issue rather
		// then disabling this in Android PFQ. At this time missing the data is allowed
		// for the grace period however it will be a build stopper after.
		Params: []testing.Param{{
			ExtraAttr:         []string{"group:arc-data-collector"},
			ExtraSoftwareDeps: []string{"android_p"},
			Val: testParam{
				vmEnabled:       false,
				upload:          true,
				dataDir:         "",
				ttsCacheEnabled: false,
			},
		}, {
			Name:              "vm",
			ExtraAttr:         []string{"group:arc-data-collector"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: testParam{
				vmEnabled:       true,
				upload:          true,
				dataDir:         "",
				ttsCacheEnabled: false,
			},
		}, {
			Name:              "local",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: testParam{
				vmEnabled:       false,
				upload:          false,
				dataDir:         "/tmp/data_collector",
				ttsCacheEnabled: true,
			},
		}, {
			Name:              "vm_local",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: testParam{
				vmEnabled:       true,
				upload:          false,
				dataDir:         "/tmp/data_collector",
				ttsCacheEnabled: false,
			},
		}},
		VarDeps: []string{"arc.perfAccountPool"},
	})
}

// DataCollector performs ARC++ boots in various conditions, grabs required data and uploads it to
// the binary server.
func DataCollector(ctx context.Context, s *testing.State) {
	const (
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

		// Number of retries for each flow in case of failure.
		// Please see b/167697547, b/181832600 for more information. Retries are
		// needed for occasional OptIn instability on ARC development builds. Only
		// lower count if for sure OptIn is completely stable.
		retryCount = 2
	)

	d := s.DUT()

	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	param := s.Param().(testParam)

	desc, err := version.GetBuildDescriptorRemotely(ctx, d, param.vmEnabled)
	if err != nil {
		s.Fatal("Failed to get ARC build desc: ", err)
	}

	v := fmt.Sprintf("%s_%s_%s", desc.CPUAbi, desc.BuildType, desc.BuildID)
	s.Logf("Detected version: %s", v)
	if desc.BuildType != "user" {
		s.Fatal("Data collector should only be run on a user build")
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

	du := dataUploader{
		ctx:             ctx,
		androidVersion:  v,
		shouldUpload:    param.upload,
		buildDescriptor: desc,
	}

	genUreadaheadPack := func() (retErr error) {
		service := arc.NewUreadaheadPackServiceClient(cl.Conn)
		// First boot is needed to be initial boot with removing all user data.
		request := arcpb.UreadaheadPackRequest{
			Creds: s.RequiredVar("arc.perfAccountPool"),
		}

		// Shorten the total context by 5 seconds to allow for cleanup.
		shortCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()

		// Limit running in PFQ for VM devices to 8GB+ RAM spec only. For local
		// test configs (upload=false) and non-VM, there are no restrictions.
		// It is known issue that 4G devices experience memory pressure during the opt in.
		// This leads to the situation when FS page caches are reclaimed and captured result
		// does not properly reflect actual FS usage. Don't upload caches to server for
		// devices lower than 8G.
		if param.vmEnabled && param.upload {
			response, err := service.CheckMinMemory(shortCtx, &empty.Empty{})
			if err != nil {
				return errors.Wrap(err, "ureadaheadPackService.CheckMinMemory returned an error")
			}
			if response.Result == false {
				testing.ContextLog(shortCtx, "Did not meet minimum memory requirement for ureadahead, skipping generate")
				return nil
			}
		}

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

		if err := du.uploadIfNeeded(targetTar, ureadAheadPack); err != nil {
			s.Fatalf("Failed to upload %q: %v", ureadAheadPack, err)
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
		defer d.Conn().CommandContext(ctx, "rm", "-rf", response.TargetDir).Output()

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

		if err := du.uploadIfNeeded(targetTar, gmsCoreCache); err != nil {
			s.Fatalf("Failed to upload %q: %v", gmsCoreCache, err)
		}

		return nil
	}

	// Helper that dumps logcat on failure. Dumping is optional and error here does not break
	// DataCollector flow and retries on error.
	dumpLogcat := func(mode string, attempt int) {
		log, err := d.Conn().CommandContext(ctx,
			"/usr/sbin/android-sh",
			"-c",
			"/system/bin/logcat -d").Output()
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

	// TODO(b/225222472): TTS Cache usage causes significant percentage of newly classified
	// ANRs in ARC. Please keep TTS Cache generation disabled by default in production or it can
	// break uprev (b/249625651) until the feature is fixed and enabled again.
	if param.ttsCacheEnabled {
		attempts = 0
		for {
			err := genTTSCache(ctx, s, cl, filepath.Join(dataDir, ttsCache), v, &du)
			if err == nil {
				break
			}
			attempts = attempts + 1
			dumpLogcat("tts", attempts)
			if attempts > retryCount {
				s.Fatal("Failed to generate TTS cache. No more retries left: ", err)
			}
			s.Log("Retrying generating TTS cache, previous attempt failed: ", err)
		}
	}
}

func genTTSCache(ctx context.Context, s *testing.State, cl *rpc.Client, targetDir, androidVersion string, du *dataUploader) error {
	service := arc.NewTTSCacheServiceClient(cl.Conn)

	// Shorten the total context by 5 seconds to allow for cleanup.
	shortCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// TTS cache setup should be disabled for the genuine cache to be generated.
	request := arcpb.TTSCacheRequest{
		TtsCacheSetupEnabled: false,
	}

	response, err := service.Generate(shortCtx, &request)
	if err != nil {
		return errors.Wrap(err, "failed to generate TTS caches")
	}
	d := s.DUT()
	defer d.Conn().CommandContext(ctx, "rm", "-rf", response.TargetDir).Output()

	if err = os.Mkdir(targetDir, 0744); err != nil {
		s.Fatalf("Failed to create %q: %v", targetDir, err)
	}

	if err = linuxssh.GetFile(shortCtx, d.Conn(), filepath.Join(response.TargetDir, response.TtsStateCacheName), filepath.Join(targetDir, response.TtsStateCacheName), linuxssh.PreserveSymlinks); err != nil {
		s.Fatalf("Failed to get %q from the device: %v", response.TtsStateCacheName, err)
	}
	targetTar := filepath.Join(targetDir, androidVersion+".tar")
	testing.ContextLogf(shortCtx, "Compressing TTS cache to %q", targetTar)
	if err = exec.Command("tar", "-cvpf", targetTar, "-C", targetDir, ".").Run(); err != nil {
		s.Fatalf("Failed to compress %q: %v", targetDir, err)
	}

	if err := du.uploadIfNeeded(targetTar, ttsCache); err != nil {
		s.Fatalf("Failed to upload %q: %v", ttsCache, err)
	}

	return nil
}

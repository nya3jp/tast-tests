// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/arc/version"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

type testParamCacheValidation struct {
	vmEnabled bool
}

const (
	// Base path
	buildsRoot = "gs://chromeos-arc-images/builds"

	// Name of jar file
	jarName = "org.chromium.arc.cachebuilder.jar"
)

// regExpEndsWithBuildID is the regexp to find the build ID from the path entry where build ID
// is the laset segment in path.
var regExpEndsWithBuildID = regexp.MustCompile(`^.+/(\d+)/$`)

// regExpLayoutEntry describes resource entry in layout.txt. For example:
// /data/user_de/0/com.google.android.gms/app_chimera/m/00000002/oat/x86_64/DynamiteLoader.vdex:644:16
var regExpLayoutEntry = regexp.MustCompile(`^(.+):([0-7]{3}):(\d+)$`)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CacheValidation,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Validates that caches match for both modes when pre-generated packages cache is enabled and disabled",
		Contacts: []string{
			"khmel@google.com",
			"arc-performance@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"arc_android_data_cros_access", "chrome"},
		ServiceDeps:  []string{"tast.cros.arc.GmsCoreCacheService", "tast.cros.arc.TTSCacheService"},
		Params: []testing.Param{{
			Name:              "pi_container",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: testParamCacheValidation{
				vmEnabled: false,
			},
		},
			{
				Name:              "r",
				ExtraSoftwareDeps: []string{"android_vm"},
				Val: testParamCacheValidation{
					vmEnabled: true,
				},
			}},
		Timeout: 10 * time.Minute,
	})
}

// findRecentBuild scans the list of available entries with ARC apps and returns one which
// has the highest build ID that indicates the most recent entry.
func findRecentBuild(ctx context.Context, vmEnabled bool) (string, error) {
	testing.ContextLogf(ctx, "Build is not official, finding the latest %q", jarName)

	branch := ""
	if vmEnabled {
		branch = "rvc-arc"
	} else {
		branch = "pi-arc"
	}

	root := fmt.Sprintf("%s/git_%s-linux-apps/", buildsRoot, branch)
	out, err := testexec.CommandContext(ctx, "gsutil", "ls", root).Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to list apps")
	}

	result := ""
	resultBuildID := 0

	for _, candidate := range strings.Split(string(out), "\n") {
		m := regExpEndsWithBuildID.FindStringSubmatch(candidate)
		// Not finding match is normal once this is external folder and may contain non-matching entries
		if m == nil {
			continue
		}
		candidateBuildID, err := strconv.Atoi(m[1])
		if err != nil {
			return "", errors.Wrapf(err, "failed to parse buildID from %s", candidate)
		}
		if candidateBuildID > resultBuildID {
			result = candidate
			resultBuildID = candidateBuildID
		}
	}

	if result == "" {
		return "", errors.Errorf("failed to find %q at %q", jarName, root)
	}

	result = result + jarName
	testing.ContextLogf(ctx, "Resolved as %q", result)
	return result, nil
}

// generateJarURL gets ARC build properties from the device, parses for build ID, and
// generates gs URL for org.chromium.ard.cachebuilder.jar
func generateJarURL(ctx context.Context, dut *dut.DUT, vmEnabled bool) (string, error) {
	desc, err := version.GetBuildDescriptorRemotely(ctx, dut, vmEnabled)
	if err != nil {
		return "", errors.Wrap(err, "failed to get ARC build desc")
	}

	if desc.Official {
		return fmt.Sprintf("%s/%s/%s/%s", buildsRoot, "git_*-linux-apps", desc.BuildID, jarName), nil
	}

	return findRecentBuild(ctx, vmEnabled)
}

func CacheValidation(ctx context.Context, s *testing.State) {
	d := s.DUT()

	param := s.Param().(testParamCacheValidation)

	desc, err := version.GetBuildDescriptorRemotely(ctx, d, param.vmEnabled)
	if err != nil {
		s.Fatal("Failed to get ARC build desc: ", err)
	}

	v := fmt.Sprintf("%s_%s_%s", desc.CPUAbi, desc.BuildType, desc.BuildID)
	s.Logf("Detected version: %s", v)
	if desc.BuildType != "user" {
		s.Fatal("Cache validation should only be run on a user build")
	}

	tempDir, err := ioutil.TempDir("", "tmp_dir")
	if err != nil {
		s.Fatal("Failed to create global temp dir: ", err)
	}
	defer os.RemoveAll(tempDir)

	url, err := generateJarURL(ctx, d, param.vmEnabled)
	if err != nil {
		s.Fatal("Failed to generate jar URL: ", err)
	}

	jarPath := filepath.Join(tempDir, filepath.Base(url))

	if err := testexec.CommandContext(ctx, "gsutil", "copy", url, jarPath).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to download from %s: %v", url, err)
	}

	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint())
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
		defer d.Conn().CommandContext(ctx, "rm", "-rf", response.TargetDir).Output()

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

	if diff, err := diffLayouts(withCacheLayout, withoutCacheLayout); err != nil {
		s.Error("Error validating app_chimera layouts: ", err)
	} else if diff != "" {
		s.Error("app_chimera layouts are different, see layout.diff")
		if err = ioutil.WriteFile(filepath.Join(s.OutDir(), "layout.diff"), []byte(diff), 0644); err != nil {
			s.Error("Failed to save layout diff: ", err)
		}
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

	s.Log("Validating TTS cache")
	withoutCacheTTSCacheFile, pregenTTSCacheFile, initializedFromCache := getTTSCache(ctx, s, cl, tempDir, false)

	if pregenTTSCacheFile == "" {
		s.Log("Pregenerated TTS cache doesn't exist, skipping TTS cache validation")
	} else {
		if initializedFromCache {
			s.Error("TTS engine should not be initialized from cache when cache setup is disabled")
		}
		if err := saveOutput(filepath.Join(s.OutDir(), "without_cache_tts_cache_diff.txt"),
			testexec.CommandContext(ctx, "diff", withoutCacheTTSCacheFile, pregenTTSCacheFile)); err != nil {
			s.Error("Error validating TTS pregenerated cache against generated cache with no cache setup: ", err)
		}

		withCacheTTSCacheFile, pregenTTSCacheFile, initializedFromCache := getTTSCache(ctx, s, cl, tempDir, true)

		if !initializedFromCache {
			s.Error("TTS engine should be initialized from cache when cache setup is enabled")
		}
		if err := saveOutput(filepath.Join(s.OutDir(), "with_cache_tts_cache_diff.txt"),
			testexec.CommandContext(ctx, "diff", withCacheTTSCacheFile, pregenTTSCacheFile)); err != nil {
			s.Error("Error validating TTS pregenerated cache against generated cache with cache setup: ", err)
		}
	}
}

func getTTSCache(ctx context.Context, s *testing.State, cl *rpc.Client, tempDir string, cacheEnabled bool) (string, string, bool) {
	service := arc.NewTTSCacheServiceClient(cl.Conn)

	// Shorten the total context by 5 seconds to allow for cleanup.
	shortCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	request := arcpb.TTSCacheRequest{
		TtsCacheSetupEnabled: cacheEnabled,
	}
	response, err := service.Generate(shortCtx, &request)
	if err != nil {
		s.Fatal(errors.Wrap(err, "failed to generate TTS cache"))
	}
	d := s.DUT()
	defer d.Conn().CommandContext(ctx, "rm", "-rf", response.TargetDir).Output()

	var subDir string
	if cacheEnabled {
		subDir = filepath.Join(tempDir, "withCache")
	} else {
		subDir = filepath.Join(tempDir, "withoutCache")
	}

	if _, err := os.Stat(subDir); os.IsNotExist(err) {
		if err := os.Mkdir(subDir, os.ModePerm); err != nil {
			s.Fatal(errors.Wrap(err, "failed to created temp dir for TTS caches"))
		}
	}

	getFile := func(file string) string {
		localFile := filepath.Join(subDir, filepath.Base(file))

		if err := d.GetFile(ctx, file, localFile); err != nil {
			s.Fatal(errors.Wrapf(err, "failed to get %q from the device", file))
		}
		return localFile
	}

	cacheFile := getFile(filepath.Join(response.TargetDir, response.TtsStateCacheName))
	pregenFile := ""
	if response.PregeneratedTtsStateCacheName != "" {
		pregenFile = getFile(filepath.Join(response.TargetDir, response.PregeneratedTtsStateCacheName))
	}

	return cacheFile, pregenFile, response.EngineInitializedFromCache
}

// resourceInfo describes attributes of file resource used for layout verification.
type resourceInfo struct {
	// Name of file resource.
	name string
	// Permission bits and file mode.
	mode os.FileMode
	// Size in blocks.
	blockSize int
}

// Matches compares two giving resources and returns true if they match. Some resources have
// specific handling.
func (r1 resourceInfo) Matches(r2 resourceInfo) bool {
	// odex files are known to be different for every generation. However besides this,
	// they may slightly change in size even on the same machine and the same build.
	// We do allow this difference. Small change usually is not reflected but may cross
	// 4K page size and in the last case this may turn to 8 block size (4K = 512*8)
	// difference.
	// TODO(khmel): Check if this sufficient.
	const odexBlocksDeltaAllowed = 8

	if r1.name != r2.name {
		return false
	}
	if r1.mode != r2.mode {
		return false
	}
	if r1.blockSize != r2.blockSize {
		d := r1.blockSize - r2.blockSize
		if d < 0 {
			d = -d
		}
		if filepath.Ext(r1.name) != ".odex" || d != odexBlocksDeltaAllowed {
			return false
		}
	}

	return true
}

// readLayout reads layout file and returns map of resources information.
func readLayout(layout string) (map[string]resourceInfo, error) {
	file, err := os.Open(layout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open")
	}
	defer file.Close()

	result := make(map[string]resourceInfo)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		m := regExpLayoutEntry.FindStringSubmatch(line)
		if m == nil {
			return nil, errors.Wrapf(err, "failed to parse layout: %q", line)
		}
		bits, err := strconv.ParseInt(m[2], 8, 16)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse permissions: %q", m[2])
		}
		blockSize, err := strconv.ParseInt(m[3], 0, 32)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse block size: %q", m[3])
		}
		result[m[1]] = resourceInfo{name: m[1], mode: os.FileMode(bits), blockSize: int(blockSize)}
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "failed to read")
	}
	return result, nil
}

// diffLayouts reads two layout files and verifies they match. In case of match, empty diff is
// returned.
func diffLayouts(path1, path2 string) (string, error) {
	layout1, err := readLayout(path1)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read layout: %q", path1)
	}
	layout2, err := readLayout(path2)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read layout: %q", path2)
	}

	diff := ""
	for k, v1 := range layout1 {
		if v2, ok := layout2[k]; ok {
			if !v1.Matches(v2) {
				diff += fmt.Sprintf("*%s %s:%d -> %s:%d\n", k, v1.mode.String(), v1.blockSize, v2.mode.String(), v2.blockSize)
			}
		} else {
			diff += fmt.Sprintf("-%s\n", k)
		}
	}
	for k := range layout2 {
		if _, ok := layout1[k]; !ok {
			diff += fmt.Sprintf("+%s\n", k)
		}
	}

	return diff, nil
}

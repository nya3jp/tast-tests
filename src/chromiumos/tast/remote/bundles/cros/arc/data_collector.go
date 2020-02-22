// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
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
		ServiceDeps:  []string{"tast.cros.arc.UreadaheadPackService"},
		Timeout:      5 * time.Minute,
		Vars: []string{
			"arc.UreadaheadService.username",
			"arc.UreadaheadService.password",
		},
	})
}

// getArcVersionRemotely gets ARC build properties from the device, parses for build ID, ABI, and
// returns these fields as a combined string.
func getArcVersionRemotely(ctx context.Context, s *testing.State, dut *dut.DUT) (string, error) {
	isARCVM, err := dut.Command("cat", "/run/chrome/is_arcvm").Output(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to check ARCVM status remotely")
	}

	var propertyFile string
	if string(isARCVM) == "1" {
		propertyFile = "/usr/share/arcvm/properties/build.prop"
	} else {
		propertyFile = "/usr/share/arc/properties/build.prop"
	}

	buildProp, err := dut.Command("cat", propertyFile).Output(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to read ARC build property file remotely")
	}
	buildPropStr := string(buildProp)

	mArch := regexp.MustCompile(`(\n|^)ro.product.cpu.abi=(.+)(\n|$)`).FindStringSubmatch(buildPropStr)
	if mArch == nil {
		return "", errors.Errorf("ro.product.cpu.abi is not found in %q", buildPropStr)
	}

	// Note, this should work on official builds only. Custom built Android image contains the
	// version in different format.
	mVersion := regexp.MustCompile(`(\n|^)ro.build.version.incremental=(\d+)(\n|$)`).FindStringSubmatch(buildPropStr)
	if mVersion == nil {
		return "", errors.Errorf("Valid ro.build.version.incremental is not found in %q", buildPropStr)
	}

	result := fmt.Sprintf("%s_%s", mArch[2], mVersion[2])
	return result, nil
}

// uploadUreadaheadPack gets ureadahead pack from the target device and uploads it to the server.
func uploadUreadaheadPack(ctx context.Context, s *testing.State, dut *dut.DUT, version, src, dst string) error {
	// Base path for uploaded ureadahead packs.
	const serverUreadaheadPackRoot = "gs://chromeos-arc-images/ureadahead_packs"

	packPath := filepath.Join(s.OutDir(), dst)
	if err := dut.GetFile(ctx, src, packPath); err != nil {
		return errors.Wrap(err, "failed to get ARC ureadahead pack from the device")
	}

	gsURL := fmt.Sprintf("%s/%s/%s", serverUreadaheadPackRoot, version, dst)

	// Use gsutil command to upload the pack to the server.
	s.Logf("Uploading ARC ureadahead pack to the server: %q", gsURL)
	cmd := exec.Command("gsutil", "copy", packPath, gsURL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed to upload ARC ureadahead pack to the server %q", out)
	}

	s.Logf("Uploaded ARC ureadahead pack to the server: %q", gsURL)
	return nil
}

// DataCollector performs ARC++ boots in various conditions, grabs required data and uploads it to
// the binary server.
func DataCollector(ctx context.Context, s *testing.State) {
	const (
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

	v, err := getArcVersionRemotely(ctx, s, d)
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

	if err = uploadUreadaheadPack(shortCtx, s, d, v, response.PackPath, initialPack); err != nil {
		s.Fatal("Failed to upload initial boot pack: ", err)
	}

	// Now pass provisioned boot and capture results.
	request.InitialBoot = false
	response, err = service.Generate(shortCtx, &request)
	if err != nil {
		s.Fatal("UreadaheadPackService.Generate returned an error for second boot pass: ", err)
	}

	if err = uploadUreadaheadPack(shortCtx, s, d, v, response.PackPath, provisionedPack); err != nil {
		s.Fatal("Failed to upload provisioned boot pack: ", err)
	}
}

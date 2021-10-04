// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package autoupdate

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/updateutil"
	"chromiumos/tast/rpc"
	aupb "chromiumos/tast/services/cros/autoupdate"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

var tlwAddress = testing.RegisterVarString("autoupdate.tlwAddress", "", "ip:port address if the TLW service")

func init() {
	testing.AddTest(&testing.Test{
		Func: NToNUpdateWithNebraska,
		Desc: "Example test for the N2N update using Nebraska and test images",
		Contacts: []string{
			"gabormagda@google.com", // Test author
		},
		Attr:         []string{}, // Manual execution only.
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps: []string{
			"tast.cros.autoupdate.NebraskaService",
			"tast.cros.autoupdate.UpdateService",
		},
		Timeout: 10 * time.Minute, // The update takes about 4 minutes.
	})
}

func NToNUpdateWithNebraska(ctx context.Context, s *testing.State) {
	if tlwAddress.Value() == "" {
		s.Logf("Start the test with `tast -var=%s=\"<tlw_ip>:<tlw_port>\" run <ip> autoupdate.NToNUpdateWithNebraskaInLab`", tlwAddress.Name())
		s.Fatalf("Unexpected %q value, this variable should not be empty", tlwAddress.Name())
	}

	originalVersion, err := updateutil.ImageVersion(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to read DUT image version before the update: ", err)
	}
	s.Log("The test is starting from image version ", originalVersion)

	builderPath, err := updateutil.ImageBuilderPath(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to read DUT image builder path before the update: ", err)
	}
	s.Log("The builder path of the image on the DUT is ", builderPath)

	// Leave 2 minutes for restart after the update.
	updateCtx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	func(ctx context.Context) {
		// Reserve cleanup time for copying the logs from the DUT.
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 1*time.Minute)
		defer cancel()

		gsPathPrefix := fmt.Sprintf("gs://chromeos-image-archive/%s", builderPath)

		url, err := updateutil.CacheForDUT(ctx, s.DUT(), tlwAddress.Value(), gsPathPrefix)
		if err != nil {
			s.Fatal("Unexpected error when caching files: ", err)
		}

		// Connect to DUT.
		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer cl.Close(cleanupCtx)

		// Create clients.
		nebraskaClient := aupb.NewNebraskaServiceClient(cl.Conn)
		updateClient := aupb.NewUpdateServiceClient(cl.Conn)

		tempDir, err := nebraskaClient.CreateTempDir(ctx, &empty.Empty{})
		if err != nil {
			s.Fatal("Failed to create temporary directory for Nebraska: ", err)
		}
		defer func(ctx context.Context) {
			if _, err := nebraskaClient.RemoveTempDir(ctx, &empty.Empty{}); err != nil {
				s.Error("Failed to remove the temporary directory: ", err)
			}
		}(cleanupCtx)

		// There is a * in the url, but it is not a wildcard for wget.
		// The * is understood by the server, and it will serve a file to download with that name.
		args := []string{
			"-P", tempDir.Path, // Download folder.
			url + "/chromeos_*_full_dev*bin.json", // Payload metadata address.
		}

		if err := s.DUT().Conn().CommandContext(ctx, "wget", args...).Run(); err != nil {
			s.Fatal("Failed to download payload metadata: ", err)
		}

		nebraska, err := nebraskaClient.Start(ctx, &aupb.StartRequest{
			Update: &aupb.Payload{
				Address:        url,
				MetadataFolder: tempDir.Path,
			},
		})
		if err != nil {
			s.Fatal("Failed to start Nebraska: ", err)
		}
		defer func(ctx context.Context) {
			if err := linuxssh.GetFile(ctx, s.DUT().Conn(), nebraska.LogPath, filepath.Join(s.OutDir(), "nebraska.log"), linuxssh.DereferenceSymlinks); err != nil {
				s.Log("Failed to save Nebraska log: ", err)
			}
		}(cleanupCtx)
		defer func(ctx context.Context) {
			if _, err := nebraskaClient.Stop(ctx, &empty.Empty{}); err != nil {
				s.Error("Failed to stop Nebraska: ", err)
			}
		}(cleanupCtx)

		// Get the update log files even if the update fails.
		defer func(ctx context.Context) {
			if err := linuxssh.GetFile(ctx, s.DUT().Conn(), "/var/log/update_engine.log", filepath.Join(s.OutDir(), "update_engine.log"), linuxssh.DereferenceSymlinks); err != nil {
				s.Log("Failed to save update engine log: ", err)
			}
		}(cleanupCtx)

		// Do the update.
		if _, err := updateClient.CheckForUpdate(ctx, &aupb.UpdateRequest{
			OmahaUrl: fmt.Sprintf("http://127.0.0.1:%s/update?critical_update=True", nebraska.Port),
		}); err != nil {
			s.Fatal("Failed to check for updates: ", err)
		}
	}(updateCtx)

	// Reboot the DUT.
	s.Log("Rebooting the DUT after the update")
	if err := s.DUT().Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot the DUT after rollback: ", err)
	}

	// Check the image version.
	version, err := updateutil.ImageVersion(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to read DUT image version after the update: ", err)
	}
	s.Logf("The DUT image version after the update is %s", version)
	if version != originalVersion {
		s.Errorf("Image version changed after the update; got %s, want %s", version, originalVersion)
	}
}

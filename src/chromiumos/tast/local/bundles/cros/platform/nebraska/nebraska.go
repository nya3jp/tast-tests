package nebraska

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// EnsureNebraskaUp prepares payloads for nebraska server and ensures nebraska
// server is up.
func EnsureNebraskaUp(ctx context.Context, dlcModuleID string, payloadData string, nebraskaPort string) (*testexec.Cmd, error) {
	const nebraskaBin = "/usr/local/bin/nebraska.py"
	pollNebraska := func() {
		// Polls nebraska until it is up and running or timeout.
		testing.Poll(ctx, func(ctx context.Context) error {
			cmd := testexec.CommandContext(ctx, "curl", "-H", "\"Accept: application/xml\"", "-H", "\"Content-Type: application/xml\"", "-X", "POST", "-d", "<request></request>", "http://127.0.0.1:2412")
			if _, err := cmd.CombinedOutput(); err != nil {
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second})
	}
	preparePayload := func(payloadDir string) error {
		parseLsbRelease := func() (string, string, error) {
			lsbReleaseFile, err := os.Open("/etc/lsb-release")
			if err != nil {
				return "", "", err
			}
			lsbReleaseContent, err := ioutil.ReadAll(lsbReleaseFile)
			if err != nil {
				return "", "", err
			}
			lsbReleaseContentSlice := strings.Split(string(lsbReleaseContent), "\n")
			appid := ""
			targetVersion := ""
			for _, val := range lsbReleaseContentSlice {
				if len(val) > 20 && val[:20] == "CHROMEOS_BOARD_APPID" {
					appid = val[21:] + "_" + dlcModuleID
				} else if len(val) > 24 && val[:24] == "CHROMEOS_RELEASE_VERSION" {
					targetVersion = val[25:]
				}
			}
			return appid, targetVersion, nil
		}
		// Creates subdirectory for install payloads.
		payloadInstallDir := filepath.Join(payloadDir, "install")
		if err := os.Mkdir(payloadInstallDir, 0755); err != nil {
			return err
		}
		// Copies DLC module payload to payload directory.
		if err := fsutil.CopyFile(payloadData, filepath.Join(payloadInstallDir, filepath.Base(payloadData))); err != nil {
			return err
		}
		// Dumps payload metadata to payload directory (parsed by nebraska server).
		appid, targetVersion, err := parseLsbRelease()
		if err != nil {
			return err
		}
		payloadManifestContent := fmt.Sprintf("{\"appid\": \"%s\",\"name\": \"%s\",\"target_version\": \"%s\",\"is_delta\": false,\"source_version\": \"0.0.0\",\"size\": 639,\"metadata_signature\": \"\",\"metadata_size\": 1,\"sha256_hex\": \"9f4290e6204eb12042b582a94a968bd565b11ae91f6bec717f0118c532293f62\"}", appid, filepath.Base(payloadData), targetVersion)
		if err := ioutil.WriteFile(filepath.Join(payloadDir, "dlc.json"), []byte(payloadManifestContent), 0755); err != nil {
			return err
		}
		return nil
	}
	// Creates directory for payloads.
	payloadDir, err := ioutil.TempDir("", "tast.platform.DLCService")
	if err != nil {
		return nil, err
	}
	// Prepares update payload.
	if err := preparePayload(payloadDir); err != nil {
		return nil, err
	}
	// Ensures Nebraska is up.
	cmd := testexec.CommandContext(ctx, "python", nebraskaBin, "--install-payloads="+payloadDir, "--port="+nebraskaPort, "--payload-addr="+"file://"+payloadDir)
	if _, err := cmd.StdinPipe(); err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	pollNebraska()
	return cmd, nil
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CryptohomeDataLeak,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verify decrypted user data is cleared after end of session",
		Contacts: []string{
			"sarthakkukreti@chromium.org", // Original autotest author
			"chingkang@google.com",        // Tast port author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func CryptohomeDataLeak(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	cryptohome := helper.CryptohomeClient()
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohome)

	var user string
	var testFile string
	func() {
		cr, err := chrome.New(ctx)
		if err != nil {
			s.Fatal("Failed to log in by Chrome: ", err)
		}
		defer cr.Close(ctx)

		user = cr.NormalizedUser()
		if mounted, err := mountInfo.IsMounted(ctx, user); err != nil {
			s.Errorf("Failed to check mounted vault for %q: %v", user, err)
		} else if !mounted {
			s.Errorf("No mounted vault for %q", user)
		}

		userHash, err := cryptohome.GetSanitizedUsername(ctx, user, true)
		testFile = fmt.Sprintf("/home/.shadow/%s/mount/hello", userHash)
		if err = ioutil.WriteFile(testFile, nil, 0666); err != nil {
			s.Fatal("Failed to create a test file: ", err)
		}

		// Check until chaps lock file disappear.
		const (
			lockDir    = "/run/lock/power_override"
			lockPrefix = "chapsd_token_init_slot_"
		)
		err = testing.Poll(ctx, func(context.Context) error {
			files, err := ioutil.ReadDir(lockDir)
			if err != nil {
				return testing.PollBreak(errors.Wrapf(err, "failed to read directory at %s", lockDir))
			}
			for _, lock := range files {
				if strings.HasPrefix(lock.Name(), lockPrefix) {
					return errors.New("lock file still exists")
				}
			}
			return nil
		}, &testing.PollOptions{
			Timeout:  30 * time.Second,
			Interval: time.Second,
		})
		if err != nil {
			s.Error("Expects chaps to finish all load events: ", err)
		}
	}()

	// Emulate logout. chrome.Chrome.Close() does not log out. So, here,
	// manually restart "ui" job for the emulation.
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to log out: ", err)
	}

	// Conceptually, this should be declared at the timing of the vault
	// creation. However, anyway removing the vault wouldn't work while
	// the user logs in. So, this is the timing to declare.
	defer cryptohome.RemoveVault(ctx, user)

	if mounted, err := mountInfo.IsMounted(ctx, user); err != nil {
		s.Errorf("Failed to check mounted vault for %q: %v", user, err)
	} else if mounted {
		s.Errorf("Mounted vault for %q is still found after logout", user)
	}

	// At this point, the session is not active and the file name is expected
	// to be encrypted again.
	_, err = os.Stat(testFile)
	if !os.IsNotExist(err) {
		s.Error("File is still visible after end of session at ", testFile)
	}
}

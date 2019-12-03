// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"strings"
	"syscall"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillInitScripts,
		Desc:     "Test that shill init scripts perform as expected",
		Contacts: []string{"arowa@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

const (
	fakeUser                 = "not-a-real-user@chromium.org"
	savedConfig              = "/tmp/network_ShillInitScripts_saved_config.tgz"
	cryptohomePathCommand    = "/usr/sbin/cryptohome-path"
	guestShillUserProfileDir = "/run/shill/guest_user_profile/shill"
	guestShillUserLogDir     = "/run/shill/guest_user_profile/shill_logs"
	magicHeader              = "# --- shill init file test magic header ---"
)

var saveDirectories = []string{"/var/cache/shill",
	"/run/shill",
	"/run/state/logged-in",
	"/run/dhcpcd",
	"/var/lib/dhcpcd"}

var rootCryptohomeDir, userCryptohomeLogDir, fakeUserHash, shillUserProfileDir, shillUserProfile string

func ShillInitScripts(ctx context.Context, s *testing.State) {
	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		s.Fatal("Failed locking the check network hook: ", err)
	}
	defer unlock()

	func() {
		defer func() {
			// Stop any shill instances started during testing.
			if err := stopShill(ctx); err != nil {
				s.Fatal("Failed stopping shill: ", err)
			}
			if err := endTest(ctx); err != nil {
				s.Fatal("Failed ending test: ", err)
			}
		}()
		// Setup the start of the test. Stop shill and create test harness.
		if err := startTest(ctx); err != nil {
			s.Fatal("Failed starting the test: ", err)
		}

		// Run test: startShill.
		if err := testStartShill(ctx); err != nil {
			s.Fatal("Failed running the test_start_shill: ", err)
		}
		if err := stopShill(ctx); err != nil {
			s.Fatal("Failed stopping shill after the test test_start_shill: ", err)
		}
		if err := eraseState(ctx); err != nil {
			s.Fatal("Failed erasing state after the test test_start_shill: ", err)
		}

		// Run test: testStartLoggedIn.
		if err := testStartLoggedIn(ctx); err != nil {
			s.Fatal("Failed on the test testStartLoggedIn: ", err)
		}
		if err := stopShill(ctx); err != nil {
			s.Fatal("Failed stopping shill after the test testStartLoggedIn: ", err)
		}
		if err := eraseState(ctx); err != nil {
			s.Fatal("Failed erasing state after the test testStartLoggedIn: ", err)
		}
	}()
}

// startTest setup the the start of the test.
func startTest(ctx context.Context) error {

	// Stop shill temporarily.
	if err := stopShill(ctx); err != nil {
		return errors.Wrap(err, "failed stopping shill")
	}

	// Deduce the root cryptohome directory name for our fake user.
	rootCryptDir, err := testexec.CommandContext(ctx, cryptohomePathCommand, "system", fakeUser).Output()
	if err != nil {
		return errors.Wrap(err, "failed getting the cryptohome directory for the fake user")
	}
	rootCryptohomeDir = string(rootCryptDir)

	// Deduce the directory for memory log storage.
	userCryptohomeLogDir = rootCryptohomeDir + "/shill_logs"

	// The sanitized hash of the username is the basename of the cryptohome.
	fakeUserHash = path.Base(rootCryptohomeDir)

	// Just in case this hash actually exists, add these to the list of
	// saved directories.
	oldLen := len(saveDirectories)
	saveDirectories = append(saveDirectories, rootCryptohomeDir)
	if len(saveDirectories) != (oldLen + 1) {
		return errors.Errorf("failed appending the root cryptohome directory %s to saved directory", saveDirectories)
	}

	// Archive the system state we will be modifying, then remove them.
	if err := testexec.CommandContext(ctx, "tar", "zcvf", savedConfig, "--directory", "/", "--ignore-failed-read", strings.Join(saveDirectories, " "), "2>/dev/null").Run(); err != nil {
		return errors.Wrap(err, "failed archiving the system state")
	}

	if err := testexec.CommandContext(ctx, "rm", "-rf", saveDirectories[0], saveDirectories[1], saveDirectories[2], saveDirectories[3], saveDirectories[4], saveDirectories[5]).Run(); err != nil {
		return errors.Wrap(err, "failed removing the system state after it was archived")
	}

	// Create the fake user's system cryptohome directory.
	if err := os.Mkdir(rootCryptohomeDir, os.ModePerm); err != nil {
		return errors.Wrap(err, "failed creating the fake user's system cryptohome directory")
	}
	shillUserProfileDir = rootCryptohomeDir + "/shill"
	shillUserProfile = shillUserProfileDir + "shill.profile"

	return nil
}

// stopShill stops shill.
func stopShill(ctx context.Context) error {
	if err := upstart.StopJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed stopping shill")
	}
	return nil
}

// endTest perfroms cleanup at the end of the test.
func endTest(ctx context.Context) error {

	if err := eraseState(ctx); err != nil {
		return errors.Wrap(err, "failed erasing the state")
	}

	if err := testexec.CommandContext(ctx, "tar", "zxvf", savedConfig, "--directory", "/").Run(); err != nil {
		return errors.Wrap(err, "failed archiving the system state")
	}

	if err := testexec.CommandContext(ctx, "rm", "-rf", savedConfig).Run(); err != nil {
		return errors.Wrap(err, "failed removing the system state after it was archived")
	}

	if err := restartSystemProcesses(ctx); err != nil {
		return errors.Wrap(err, "failed restarting shill")
	}
	return nil
}

// eraseState removes all the test harness files.
func eraseState(ctx context.Context) error {
	if err := testexec.CommandContext(ctx, "rm", "-rf", saveDirectories[0], saveDirectories[1], saveDirectories[2], saveDirectories[3], saveDirectories[4], saveDirectories[5]).Run(); err != nil {
		return errors.Wrap(err, "failed removing the system state")
	}

	if err := os.Mkdir(rootCryptohomeDir, os.ModePerm); err != nil {
		return errors.Wrapf(err, "failed making the directory: %s", rootCryptohomeDir)
	}
	return nil
}

// restartSystemProcesses restarts shill.
func restartSystemProcesses(ctx context.Context) error {
	if err := upstart.RestartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed restarting shill")
	}
	return nil
}

// testStartShill tests all created path names during shill startup.
func testStartShill(ctx context.Context) error {

	if err := startShill(ctx); err != nil {
		return err
	}

	if err := assureIsDir(ctx, "/run/shill"); err != nil {
		return err
	}
	if err := assureIsDir(ctx, "/var/lib/dhcpcd"); err != nil {
		return err
	}
	if err := assurePathOwner(ctx, "/var/lib/dhcpcd", "dhcp"); err != nil {
		return err
	}
	if err := assurePathGroup(ctx, "/var/lib/dhcpcd", "dhcp"); err != nil {
		return err
	}
	if err := assureIsDir(ctx, "/run/dhcpcd"); err != nil {
		return err
	}
	if err := assurePathOwner(ctx, "/run/dhcpcd", "dhcp"); err != nil {
		return err
	}
	if err := assurePathGroup(ctx, "/run/dhcpcd", "dhcp"); err != nil {
		return err
	}
	return nil
}

// startShill starts shill.
func startShill(ctx context.Context) error {
	if err := upstart.StartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed starting shill")
	}
	return nil
}

// assureIsDir asserts that |path| is a directory.
func assureIsDir(ctx context.Context, path string) error {
	if err := assureExists(ctx, path); err != nil {
		return errors.Wrapf(err, "failed path %s doesn't exist", path)
	}

	if stat, err := os.Stat(path); err != nil || !(stat.IsDir()) {
		return errors.Wrapf(err, "failed path %s is not a directory", path)
	}
	return nil
}

// assureExists asserts that |path| exists.
func assureExists(ctx context.Context, path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return err
	}
	return nil
}

// assurePathOwner asserts that |path| is owned by |owner|.
func assurePathOwner(ctx context.Context, path string, owner string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return errors.Wrapf(err, "failed getting FileInfo interface of the path %s: ", path)
	}
	sys := stat.Sys().(*syscall.Stat_t)
	userID := fmt.Sprint(sys.Uid)
	userStruct, err := user.LookupId(userID)
	if err != nil {
		return errors.Wrapf(err, "failed getting the User struct from the user id %s of the path %s: ", userID, path)
	}
	userName := userStruct.Username
	if userName != owner {
		return errors.Errorf("found unexpected (login name/ owner): got %s, want %s", userName, owner)
	}
	return nil
}

// assurePathGroup asserts that |path| is owned by |group|.
func assurePathGroup(ctx context.Context, path string, group string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return errors.Wrapf(err, "failed getting FileInfo interface of the path %s: ", path)
	}
	sys := stat.Sys().(*syscall.Stat_t)
	groupID := fmt.Sprint(sys.Gid)
	groupStruct, err := user.LookupGroupId(groupID)
	if err != nil {
		return errors.Wrapf(err, "failed getting the Group struct from the group ID %s of the path %s: ", groupID, path)
	}
	groupName := groupStruct.Name
	if groupName != group {
		return errors.Errorf("found unexpected group name: got %s, want %s", groupName, group)
	}
	return nil
}

// testStartLoggedIn tests starting up shill while user is already logged in.
func testStartLoggedIn(ctx context.Context) error {
	if err := os.Mkdir("/run/shill", os.ModePerm); err != nil {
		return errors.Wrap(err, "failed making the directory /run/shill")
	}
	if err := os.Mkdir("/run/shill/user_profiles", os.ModePerm); err != nil {
		return errors.Wrap(err, "failed making the directory /run/shill/user_profiles")
	}
	if err := createShillUserProfile(ctx, " "); err != nil {
		return errors.Wrap(err, "failed creating the shill user profile")
	}
	if err := os.Symlink(shillUserProfileDir, "/run/shill/user_profiles/chronos"); err != nil {
		return errors.Wrapf(err, "failed to Symlink %s to /run/shill/user_profiles/chronos", shillUserProfileDir)
	}
	if err := touch(ctx, "/run/state/logged-in"); err != nil {
		return err
	}
	if err := startShill(ctx); err != nil {
		return err
	}
	if err := os.Remove("/run/state/logged-in"); err != nil {
		return errors.Wrap(err, "failed to unlink/remove /run/state/logged-in")
	}
	return nil
}

// createShillUserProfile creates a fake user profile with |contents|.
func createShillUserProfile(ctx context.Context, contents string) error {
	if err := os.Mkdir(shillUserProfileDir, os.ModePerm); err != nil {
		return errors.Wrapf(err, "failed making the directory: %s", shillUserProfileDir)
	}
	if err := createFileWithContents(ctx, shillUserProfile, contents); err != nil {
		return errors.Wrapf(err, "failed creating the file: %s", shillUserProfile)
	}
	return nil
}

// createFileWithContents creates a file named |filename| that contains |contents|.
func createFileWithContents(ctx context.Context, fileName string, contents string) error {
	if err := ioutil.WriteFile(fileName, []byte(contents), 0644); err != nil {
		return err
	}
	return nil
}

// touch creates an empty file named |filename|.
func touch(ctx context.Context, filename string) error {
	if err := createFileWithContents(ctx, filename, ""); err != nil {
		return errors.Wrapf(err, "failed creating an empty file: %s", filename)
	}
	return nil
}

// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"bytes"
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

	defer func() {
		// Stop any shill instances started during testing.
		if err := stopShill(ctx); err != nil {
			s.Fatal("Failed stopping shill: ", err)
		}
		if err := endTest(ctx); err != nil {
			s.Fatal("Failed ending test: ", err)
		}
	}()

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

	// Run test: testLogin.
	// Login should create a profile directory, then create and push
	// a user profile, given no previous state.
	if err := startShill(ctx); err != nil {
		s.Fatal("Failed starting shill: ", err)
	}
	if err := testLogin(ctx); err != nil {
		s.Fatal("Failed on the test testLogin: ", err)
	}
	if err := stopShill(ctx); err != nil {
		s.Fatal("Failed stopping shill after the test testLogin: ", err)
	}
	if err := eraseState(ctx); err != nil {
		s.Fatal("Failed erasing state after the test testLogin: ", err)
	}

	// Run test: testLoginGuest.
	// Login should create a temporary profile directory in /run,
	// instead of using one within the root directory for normal users.
	if err := startShill(ctx); err != nil {
		s.Fatal("Failed starting shill: ", err)
	}
	if err := testLoginGuest(ctx); err != nil {
		s.Fatal("Failed on the test testLoginGuest: ", err)
	}
	if err := stopShill(ctx); err != nil {
		s.Fatal("Failed stopping shill after the test testLoginGuest: ", err)
	}
	if err := eraseState(ctx); err != nil {
		s.Fatal("Failed erasing state after the test testLoginGuest: ", err)
	}

	// Run test: testLoginProfileExists.
	// Login script should only push (and not create) the user profile
	// if a user profile already exists.
	if err := startShill(ctx); err != nil {
		s.Fatal("Failed starting shill: ", err)
	}
	if err := testLoginProfileExists(ctx); err != nil {
		s.Fatal("Failed on the test testLoginProfileExists: ", err)
	}
	if err := stopShill(ctx); err != nil {
		s.Fatal("Failed stopping shill after the test testLoginProfileExists: ", err)
	}
	if err := eraseState(ctx); err != nil {
		s.Fatal("Failed erasing state after the test testLoginProfileExists: ", err)
	}

	// Run test: testLoginMultiProfile.
	// Login script should not create multiple profiles in parallel
	// if called more than once without an intervening logout.  Only
	// the initial user profile should be created.
	if err := startShill(ctx); err != nil {
		s.Fatal("Failed starting shill: ", err)
	}
	if err := testLoginMultiProfile(ctx); err != nil {
		s.Fatal("Failed on the test testLoginMultiProfile: ", err)
	}
	if err := stopShill(ctx); err != nil {
		s.Fatal("Failed stopping shill after the test testLoginMultiProfile: ", err)
	}
	if err := eraseState(ctx); err != nil {
		s.Fatal("Failed erasing state after the test testLoginMultiProfile: ", err)
	}

	// Run test: testLogout.
	if err := startShill(ctx); err != nil {
		s.Fatal("Failed starting shill: ", err)
	}
	if err := testLogout(ctx); err != nil {
		s.Fatal("Failed on the test testLogout: ", err)
	}
	if err := stopShill(ctx); err != nil {
		s.Fatal("Failed stopping shill after the test testLogout: ", err)
	}
	if err := eraseState(ctx); err != nil {
		s.Fatal("Failed erasing state after the test testLogout: ", err)
	}

}

// startTest setup the start of the test. Stop shill and create test harness.
func startTest(ctx context.Context) error {
	// Stop shill temporarily.
	if err := stopShill(ctx); err != nil {
		return err
	}

	// Deduce the root cryptohome directory name for our fake user.
	rootCryptDir, err := testexec.CommandContext(ctx, cryptohomePathCommand, "system", fakeUser).Output()
	if err != nil {
		return errors.Wrap(err, "failed getting the cryptohome directory for the fake user")
	}

	// delete the "\n" at the end of the root Cryptohome Directory.
	rootCryptDir = bytes.Trim(rootCryptDir, "\n")

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
		return errors.Errorf("failed appending the root cryptohome directory %s to saved directory", rootCryptohomeDir)
	}

	// Archive the system state we will be modifying, then remove them.
	if err := testexec.CommandContext(ctx, "tar", "zcvf", savedConfig, "--directory", "/", "--ignore-failed-read", strings.Join(saveDirectories, " "), "2>/dev/null").Run(); err != nil {
		return errors.Wrap(err, "failed archiving the system state")
	}

	if err := testexec.CommandContext(ctx, "rm", "-rf", saveDirectories[0], saveDirectories[1], saveDirectories[2], saveDirectories[3], saveDirectories[4], saveDirectories[5]).Run(); err != nil {
		return errors.Wrap(err, "failed removing the system state after it was archived")
	}

	// Create the fake user's system cryptohome directory.
	if err := os.Mkdir(rootCryptohomeDir, 0777); err != nil {
		return errors.Wrapf(err, "failed making the directory after removing the system state: %s", rootCryptohomeDir)
	}

	shillUserProfileDir = rootCryptohomeDir + "/shill"
	shillUserProfile = shillUserProfileDir + "/shill.profile"

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
	if len(saveDirectories) == 6 {
		if err := testexec.CommandContext(ctx, "rm", "-rf", saveDirectories[0], saveDirectories[1], saveDirectories[2], saveDirectories[3], saveDirectories[4], saveDirectories[5]).Run(); err != nil {
			return errors.Wrap(err, "failed removing the system state")
		}
	} else if len(saveDirectories) == 5 {
		if err := testexec.CommandContext(ctx, "rm", "-rf", saveDirectories[0], saveDirectories[1], saveDirectories[2], saveDirectories[3], saveDirectories[4]).Run(); err != nil {
			return errors.Wrap(err, "failed removing the system state")
		}
	} else {
		return errors.Errorf("found unexpected saved directories array size: got %v, want 6 or 5", len(saveDirectories))
	}

	if err := os.Mkdir(rootCryptohomeDir, 0777); err != nil {
		return errors.Wrapf(err, "failed making the directory after removing the system state: %s", rootCryptohomeDir)
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
		return errors.Wrap(err, "failed asserting that /run/shill is a directory")
	}
	if err := assureIsDir(ctx, "/var/lib/dhcpcd"); err != nil {
		return errors.Wrap(err, "failed asserting that /var/lib/dhcpcd is a directory")
	}
	if err := assurePathOwner(ctx, "/var/lib/dhcpcd", "dhcp"); err != nil {
		return errors.Wrap(err, "failed asserting that the user owner of the path /run/lib/dhcpcd is dhcp")
	}
	if err := assurePathGroup(ctx, "/var/lib/dhcpcd", "dhcp"); err != nil {
		return errors.Wrap(err, "failed asserting that the group owner of the path /run/lib/dhcpcd is dhcp")
	}
	if err := assureIsDir(ctx, "/run/dhcpcd"); err != nil {
		return errors.Wrap(err, "failed asserting that /run/dhcpcd is a directory")
	}
	if err := assurePathOwner(ctx, "/run/dhcpcd", "dhcp"); err != nil {
		return errors.Wrap(err, "failed asserting that the user owner of the path /run/dhcpcd is dhcp")
	}
	if err := assurePathGroup(ctx, "/run/dhcpcd", "dhcp"); err != nil {
		return errors.Wrap(err, "failed asserting that the group owner of the path /run/dhcpcd is dhcp")
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

// assureNotExists asserts that |path| doesn't exist.
func assureNotExists(ctx context.Context, path string) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
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

	if err := createShillUserProfile(ctx, ""); err != nil {
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
		return errors.Wrap(err, "failed to remove /run/state/logged-in")
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

// testLogin tests the login process.
func testLogin(ctx context.Context) error {
	if err := login(ctx, fakeUser); err != nil {
		return err
	}

	if err := assureExists(ctx, shillUserProfile); err != nil {
		return errors.Wrapf(err, "failed shill user profile %s doesn't exist", shillUserProfile)
	}

	if err := assureIsDir(ctx, shillUserProfileDir); err != nil {
		return errors.Wrapf(err, "failed asserting that %v is a directory", shillUserProfileDir)
	}

	if err := assureIsDir(ctx, "/run/shill/user_profiles"); err != nil {
		return errors.Wrap(err, "failed asserting that /run/shill/user_profiles is a directory")
	}

	if err := assureIsLinkTo(ctx, "/run/shill/user_profiles/chronos", shillUserProfileDir); err != nil {
		return err
	}

	if err := assureIsDir(ctx, userCryptohomeLogDir); err != nil {
		return errors.Wrapf(err, "failed asserting that %v is a directory", userCryptohomeLogDir)
	}

	if err := assureIsLinkTo(ctx, "/run/shill/log", userCryptohomeLogDir); err != nil {
		return err
	}

	return nil
}

// login simulates the login process.
func login(ctx context.Context, user string) error {
	result, err := hasSystemd(ctx)
	if err != nil {
		return errors.Wrap(err, "failed checking for existance of systemd")
	}

	chromeUser := "CHROMEOS_USER=" + user

	if result {
		if err := testexec.CommandContext(ctx, "systemctl", "set-environment", chromeUser, "&&", "systemctl", "start", "shill-start-user-session").Run(); err != nil {
			return err
		}
	} else {

		if err := testexec.CommandContext(ctx, "start", "shill-start-user-session", chromeUser).Run(); err != nil {
			return err
		}
	}
	return nil
}

// logout simulates the logout process.
func logout(ctx context.Context) error {

	if err := testexec.CommandContext(ctx, "start", "shill-stop-user-session").Run(); err != nil {
		return err
	}

	return nil
}

// hasSystemd checks if the host is running systemd.
func hasSystemd(ctx context.Context) (bool, error) {
	rel, err := os.Readlink("/proc/1/exe")
	if err != nil {
		return false, errors.Wrap(err, "failed to readlink /proc/1/exe")
	}

	return (path.Base(rel) == "systemd"), nil
}

// assureIsLinkTo asserts that |path| is a symbolic link to |pointee|.
func assureIsLinkTo(ctx context.Context, path string, pointee string) error {
	if err := assureIsLink(ctx, path); err != nil {
		return err
	}

	rel, err := os.Readlink(path)
	if err != nil {
		return errors.Wrapf(err, "failed to readlink: %v", path)
	}

	if rel != pointee {
		return errors.Errorf("found unexpected profile path: got %v, want %v", rel, pointee)
	}

	return nil
}

// assureIsLink asserts that |path| is a symbolic link.
func assureIsLink(ctx context.Context, path string) error {
	if err := assureExists(ctx, path); err != nil {
		return errors.Wrapf(err, "failed path %s doesn't exist", path)
	}

	fileInfoStat, err := os.Lstat(path)
	if err != nil {
		return err
	}

	if fileInfoStat.Mode()&os.ModeSymlink != os.ModeSymlink {
		return errors.Errorf("found unexpected file mode: got %v, want %v", fileInfoStat.Mode(), os.ModeSymlink)
	}

	return nil
}

// testLoginGuest tests the guest login process.
func testLoginGuest(ctx context.Context) error {
	// Simulate guest login.
	// For guest login, session-manager passes an empty CHROMEOS_USER arg.
	if err := login(ctx, ""); err != nil {
		return err
	}

	if err := assureNotExists(ctx, shillUserProfile); err != nil {
		return errors.Wrapf(err, "failed shill user profile %s does exist", shillUserProfile)
	}

	if err := assureNotExists(ctx, shillUserProfileDir); err != nil {
		return errors.Wrapf(err, "failed shill user profile %s does exist", shillUserProfile)
	}

	if err := assureIsDir(ctx, guestShillUserProfileDir); err != nil {
		return errors.Wrapf(err, "failed asserting that %v is a directory", guestShillUserProfileDir)
	}

	if err := assureIsDir(ctx, "/run/shill/user_profiles"); err != nil {
		return errors.Wrap(err, "failed asserting that /run/shill/user_profiles is a directory")
	}

	if err := assureIsLinkTo(ctx, "/run/shill/user_profiles/chronos", guestShillUserProfileDir); err != nil {
		return err
	}

	if err := assureIsDir(ctx, guestShillUserLogDir); err != nil {
		return errors.Wrapf(err, "failed asserting that %v is a directory", guestShillUserLogDir)
	}

	if err := assureIsLinkTo(ctx, "/run/shill/log", guestShillUserLogDir); err != nil {
		return err
	}

	return nil
}

// testLoginProfileExists tests logging in a user whose profile already exists.
func testLoginProfileExists(ctx context.Context) error {

	if err := os.Mkdir(shillUserProfileDir, 0777); err != nil {
		return errors.Wrapf(err, "failed creating the directory: %s", shillUserProfileDir)
	}
	if err := touch(ctx, shillUserProfile); err != nil {
		return err
	}
	if err := login(ctx, fakeUser); err != nil {
		return errors.Wrap(err, "failed logging in")
	}

	return nil
}

// testLogout tests the logout process.
func testLogout(ctx context.Context) error {
	if err := os.MkdirAll("/run/shill/user_profiles", 0777); err != nil {
		return errors.Wrap(err, "failed creating the directory: /run/shill/user_profiles")
	}

	if err := os.MkdirAll(guestShillUserProfileDir, 0777); err != nil {
		return errors.Wrapf(err, "failed creating the directory: %s", guestShillUserProfileDir)
	}

	if err := os.MkdirAll(guestShillUserLogDir, 0777); err != nil {
		return errors.Wrapf(err, "failed creating the directory: %s", guestShillUserLogDir)
	}

	if err := touch(ctx, "/run/state/logged-in"); err != nil {
		return err
	}

	if err := logout(ctx); err != nil {
		return errors.Wrap(err, "failed logging out")
	}

	if err := assureNotExists(ctx, "/run/shill/user_profiles"); err != nil {
		return errors.Wrap(err, "failed shill user profile /run/shill/user_profiles does exist")
	}

	if err := assureNotExists(ctx, guestShillUserProfileDir); err != nil {
		return errors.Wrapf(err, "failed guest shill user profile directory %s does exist", guestShillUserProfileDir)
	}

	if err := assureNotExists(ctx, guestShillUserLogDir); err != nil {
		return errors.Wrapf(err, "failed guest shill user log directory %s does exist", guestShillUserLogDir)
	}

	return nil
}

// testLoginMultiProfile tests signalling shill about multiple logged-in users.
func testLoginMultiProfile(ctx context.Context) error {
	if err := createShillUserProfile(ctx, ""); err != nil {
		return err
	}

	// First logged-in user should create a profile (tested above).
	if err := login(ctx, fakeUser); err != nil {
		return errors.Wrap(err, "failed logging in")
	}

	for i := 0; i < 5; i++ {
		if err := login(ctx, fakeUser); err != nil {
			return errors.Wrap(err, "failed logging in")
		}
		files, err := ioutil.ReadDir("/run/shill/user_profiles")
		if err != nil {
			return err
		}
		if len(files) > 1 || len(files) == 0 {
			return errors.Errorf("found unexpected number of profiles in the directorey /run/shill/user_profiles: got %v, want 1 ", len(files))
		}
		if files[0].Name() != "chronos" {
			return errors.Errorf("found unexpected profile link in the directorey /run/shill/user_profiles: got %v, want chronos ", files[0].Name())
		}
		if err := assureIsLinkTo(ctx, "/run/shill/log", userCryptohomeLogDir); err != nil {
			return err
		}
	}

	return nil
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillInitScripts,
		Desc:     "Test that shill init scripts perform as expected",
		Contacts: []string{"arowa@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

const (
	fakeUser                 = "not-a-real-user@chromium.org"
	savedConfig              = "/tmp/network_ShillInitScripts_saved_config.tgz"
	cryptohomePathCommand    = "/usr/sbin/cryptohome-path"
	guestShillUserProfileDir = "/run/shill/guest_user_profile/shill"
	guestShillUserLogDir     = "/run/shill/guest_user_profile/shill_logs"
	chronosProfileName       = "~chronos/shill"
	expectedProfileName      = "/profile/chronos/shill"
	shillPollingTimeout      = 10 * time.Second
	timeDelay                = 1 * time.Second
	dbusMonitorTimeout       = 5 * time.Second
	createUserProfile        = "CreateProfile"
	insertUserProfile        = "InsertUserProfile"
	popAllUserProfiles       = "PopAllUserProfiles"
	dummyEndSignal           = "DummyEndSignal"
	dummyStartSignal         = "DummyStartSignal"
)

type testEnv struct {
	rootCryptohomeDir    string
	userCryptohomeLogDir string
	shillUserProfileDir  string
	shillUserProfile     string
	saveDirectories      []string
}

//testFuncType  testFuncType take an context.Context, testEnv struct, and return a error.
type testFuncType func(ctx context.Context, env *testEnv) error

func ShillInitScripts(ctx context.Context, s *testing.State) {
	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		s.Fatal("Failed locking the check network hook: ", err)
	}
	defer unlock()

	var env testEnv

	defer tearDown(ctx, &env)

	if err := setUp(ctx, &env); err != nil {
		s.Fatal("Failed starting the test: ", err)
	}

	var testName = []string{"testStartShill", "testStartLoggedIn", "testLogin", "testLoginGuest", "testLoginProfileExists", "testLogout", "testLoginMultiProfile"}
	var testFunc = []testFuncType{testStartShill, testStartLoggedIn, testLogin, testLoginGuest, testLoginProfileExists, testLogout, testLoginMultiProfile}
	// Run all the tests.
	for i := 0; i < len(testName); i++ {
		if err := runTest(ctx, &env, testName[i], testFunc[i]); err != nil {
			s.Fatalf("Failed running the %s: %v", testName[i], err)
		}
	}
}

// runTest runs the test, stops shill and erase the state.
func runTest(ctx context.Context, env *testEnv, name string, fn testFuncType) error {
	if err := fn(ctx, env); err != nil {
		return err
	}
	if err := stopShill(ctx); err != nil {
		return errors.Wrapf(err, "failed stopping shill after the test: %s", name)
	}
	if err := eraseState(ctx, env); err != nil {
		return errors.Wrapf(err, "failed erasing state after the test: %s", name)
	}
	return nil
}

// setUp setup the start of the test. Stop shill and create test harness.
func setUp(ctx context.Context, env *testEnv) error {
	// The directories names that are created during the test and deleted at the end of the test.
	env.saveDirectories = append(env.saveDirectories, "/var/cache/shill", "/run/shill", "/run/state/logged-in", "/run/dhcpcd", "/var/lib/dhcpcd")

	// Stop shill temporarily.
	if err := stopShill(ctx); err != nil {
		return err
	}

	// Deduce the root cryptohome directory name for our fake user.
	rootCryptDir, err := testexec.CommandContext(ctx, cryptohomePathCommand, "system", fakeUser).Output()
	if err != nil {
		return errors.Wrap(err, "failed getting the cryptohome directory for the fake user")
	}

	// Delete the "\n" at the end of the root Cryptohome Directory.
	rootCryptDir = bytes.Trim(rootCryptDir, "\n")

	env.rootCryptohomeDir = string(rootCryptDir)

	// Deduce the directory for memory log storage.
	env.userCryptohomeLogDir = filepath.Join(env.rootCryptohomeDir, "/shill_logs")

	// Just in case this hash actually exists, add these to the list of
	// saved directories.
	env.saveDirectories = append(env.saveDirectories, env.rootCryptohomeDir)

	// Archive the system state we will be modifying, then remove them.
	if err := testexec.CommandContext(ctx, "tar", "zcvf", savedConfig, "--directory", "/", "--ignore-failed-read", strings.Join(env.saveDirectories, " ")).Run(); err != nil {
		return errors.Wrap(err, "failed archiving the system state")
	}

	rmArgs := []string{"-rf"}
	rmArgs = append(rmArgs, env.saveDirectories...)
	if err := testexec.CommandContext(ctx, "rm", rmArgs...).Run(); err != nil {
		return errors.Wrap(err, "failed removing the system state after it was archived")
	}

	// Create the fake user's system cryptohome directory.
	if err := os.Mkdir(env.rootCryptohomeDir, 0777); err != nil {
		return errors.Wrapf(err, "failed making the directory after removing the system state: %s", env.rootCryptohomeDir)
	}

	env.shillUserProfileDir = filepath.Join(env.rootCryptohomeDir, "/shill")
	env.shillUserProfile = filepath.Join(env.shillUserProfileDir, "/shill.profile")

	return nil
}

// stopShill stops shill.
func stopShill(ctx context.Context) error {
	if err := upstart.StopJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed stopping shill")
	}
	return waitShillStopped(ctx)
}

// startShill starts shill.
func startShill(ctx context.Context) error {
	if err := upstart.StartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed starting shill")
	}
	return waitShillStarted(ctx)
}

// restartShill restarts shill.
func restartShill(ctx context.Context) error {
	if err := upstart.RestartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed restarting shill")
	}
	return waitShillStarted(ctx)
}

// findPid retuen the process id of the |process name|.
func findPid(ctx context.Context, processName string) string {
	processID, _ := testexec.CommandContext(ctx, "pgrep", processName).Output()
	return string(processID)
}

// hasShillStopped checks if shill has stopped.
func hasShillStopped(ctx context.Context) error {
	if findPid(ctx, "shill") != "" {
		return errors.New("failed shill process is not yet stopped")
	}
	return nil
}

// hasShillStarted checks if shill has started.
func hasShillStarted(ctx context.Context) error {
	if findPid(ctx, "shill") == "" {
		return errors.New("failed shill process is not started yet")
	}
	return nil
}

// waitShillStopped checks if shill has stoppped.
func waitShillStopped(ctx context.Context) error {
	return testing.Poll(ctx, func(ctx context.Context) (e error) {
		return hasShillStopped(ctx)
	}, &testing.PollOptions{Timeout: shillPollingTimeout})
}

// waitShillStarted checks if shill has started.
func waitShillStarted(ctx context.Context) error {
	return testing.Poll(ctx, func(ctx context.Context) (e error) {
		return hasShillStarted(ctx)
	}, &testing.PollOptions{Timeout: shillPollingTimeout})
}

// tearDown performs cleanup at the end of the test.
func tearDown(ctx context.Context, env *testEnv) error {
	var errMsg error
	errMsg = nil
	// Stop any shill instances started during testing.
	if err := stopShill(ctx); err != nil {
		errMsg = errors.Wrap(errMsg, errors.Wrap(err, "failed stopping shill").Error())
	}
	if err := eraseState(ctx, env); err != nil {
		errMsg = errors.Wrap(errMsg, errors.Wrap(err, "failed erasing the state").Error())
	}

	if err := testexec.CommandContext(ctx, "tar", "zxvf", savedConfig, "--directory", "/").Run(); err != nil {
		errMsg = errors.Wrap(errMsg, errors.Wrap(err, "failed archiving the system state").Error())
	}

	if err := testexec.CommandContext(ctx, "rm", "-rf", savedConfig).Run(); err != nil {
		errMsg = errors.Wrap(errMsg, errors.Wrap(err, "failed removing the system state after it was archived").Error())
	}

	if err := restartShill(ctx); err != nil {
		errMsg = errors.Wrap(errMsg, errors.Wrap(err, "failed restarting shill").Error())
	}

	return errMsg
}

// eraseState removes all the test harness files.
func eraseState(ctx context.Context, env *testEnv) error {
	rmArgs := []string{"-rf"}
	rmArgs = append(rmArgs, env.saveDirectories...)
	if err := testexec.CommandContext(ctx, "rm", rmArgs...).Run(); err != nil {
		return errors.Wrap(err, "failed removing the system state")
	}
	if err := os.Mkdir(env.rootCryptohomeDir, 0777); err != nil {
		return errors.Wrapf(err, "failed making the directory after removing the system state: %s", env.rootCryptohomeDir)
	}
	return nil
}

// assureIsDir asserts that |path| is a directory.
func assureIsDir(path string) error {
	if err := assureExists(path); err != nil {
		return err
	}

	stat, err := os.Stat(path)
	if err != nil {
		return errors.Wrapf(err, "failed getting the file info struct of the path: %s", path)
	}
	if !stat.IsDir() {
		return errors.Errorf("failed path is not a directory: %s", path)
	}

	return nil
}

// assureExists asserts that |path| exists.
func assureExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return err
	}
	return nil
}

// assureNotExists asserts that |path| doesn't exist.
func assureNotExists(path string) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	return errors.Errorf("unexpected path: %s", path)
}

// assurePathOwner asserts that |path| is owned by |owner|.
func assurePathOwner(path string, owner string) error {
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
func assurePathGroup(path string, group string) error {
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

// createShillUserProfile creates a fake user profile with |contents|.
func createShillUserProfile(contents string, env *testEnv) error {
	if err := os.Mkdir(env.shillUserProfileDir, os.ModePerm); err != nil {
		return errors.Wrapf(err, "failed making the directory: %s", env.shillUserProfileDir)
	}
	if err := createFileWithContents(env.shillUserProfile, contents); err != nil {
		return errors.Wrapf(err, "failed creating the file: %s", env.shillUserProfile)
	}
	return nil
}

// createFileWithContents creates a file named |filename| that contains |contents|.
func createFileWithContents(fileName string, contents string) error {
	if err := ioutil.WriteFile(fileName, []byte(contents), 0644); err != nil {
		return err
	}
	return nil
}

// touch creates an empty file named |filename|.
func touch(filename string) error {
	if err := createFileWithContents(filename, ""); err != nil {
		return errors.Wrapf(err, "failed creating an empty file: %s", filename)
	}
	return nil
}

// login simulates the login process.
func login(ctx context.Context, user string) error {
	chromeUser := "CHROMEOS_USER=" + user

	if err := testexec.CommandContext(ctx, "start", "shill-start-user-session", chromeUser).Run(); err != nil {
		return err
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
func hasSystemd() (bool, error) {
	rel, err := os.Readlink("/proc/1/exe")
	if err != nil {
		return false, errors.Wrap(err, "failed to readlink /proc/1/exe")
	}

	return (path.Base(rel) == "systemd"), nil
}

// assureIsLinkTo asserts that |path| is a symbolic link to |pointee|.
func assureIsLinkTo(path string, pointee string) error {
	if err := assureIsLink(path); err != nil {
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
func assureIsLink(path string) error {
	if err := assureExists(path); err != nil {
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

// getProfileList return the profiles in the profile stack.
func getProfileList(ctx context.Context) ([]dbus.ObjectPath, error) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed creating shill manger object")
	}
	// Refresh the in-memory profile list.
	if _, err := manager.GetProperties(ctx); err != nil {
		return nil, errors.Wrap(err, "failed refreshing the in-memeory profile list")
	}

	// Get current profiles.
	profiles, err := manager.GetProfiles(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed getting profile list")
	}

	return profiles, nil
}

// createProfile creates a new shill profile.
func createProfile(ctx context.Context, profileName string) error {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed creating shill manger object")
	}

	if _, err := manager.CreateProfile(ctx, chronosProfileName); err != nil {
		return errors.Wrapf(err, "failed creating profile: %v", chronosProfileName)
	}
	return nil
}

// dbusEventMonitor monitors the system message bus for those D-Bus calls we want to observe (defined in isWhitelist()).
// It returns a stop function and error channel. The stop function stops the D-Bus monitor and return the called methods and/or error.
func dbusEventMonitor(ctx context.Context) (func() ([]string, error), chan error) {
	ch := make(chan error, 1)
	chStartAction := make(chan error, 1)
	var calledMethods []string
	stop := func() ([]string, error) {
		// Send a dummy dbus signal to stop the dbus-monitor.
		if err := testexec.CommandContext(ctx, "dbus-send", "--system", "/", "com.dummy.DummyEndSignal").Run(); err != nil {
			return calledMethods, errors.Wrap(err, "failed sending dummy signal to end the dbusEventMonitor")
		}
		if err := <-ch; err != nil {
			return calledMethods, err
		}
		return calledMethods, nil
	}

	cmd := testexec.CommandContext(ctx, "dbus-monitor", "--system")
	cmdOut, err := cmd.StdoutPipe()
	if err != nil {
		chStartAction <- errors.Wrap(err, "failed to get stdout reader")
		return stop, chStartAction
	}
	if err := cmd.Start(); err != nil {
		chStartAction <- errors.Wrap(err, "failed to spawn \"dbus monitor\"")
		return stop, chStartAction
	}

	// Spawn watch routine.
	go func() {
		defer func() {
			// Always try to stop the dbus monitor before leaving.
			if err := cmd.Kill(); err != nil {
				testing.ContextLog(ctx, "Failed to kill dbus monitor")
			}
			cmd.Wait()
		}()

		calledMethods = nil
		startMonitor := false
		firstScan := true
		scanner := bufio.NewScanner(cmdOut)
		for scanner.Scan() {
			if firstScan {
				// Send a dummy dbus signal that is used to start monitoring the D-Bus calls we want to observe.
				if err := testexec.CommandContext(ctx, "dbus-send", "--system", "/", "com.dummy.DummyStartSignal").Run(); err != nil {
					chStartAction <- errors.Wrap(err, "failed sending dummy signal to start monitoring the D-Bus calls we want to observe")
					return
				}
				firstScan = false
			}
			line := scanner.Text()
			isValid, dbusCmd := inWhitelist(line)
			if isValid {
				if startMonitor {
					if dbusCmd == dummyEndSignal {
						ch <- nil
						return
					}
					calledMethods = append(calledMethods, dbusCmd)
				} else if dbusCmd == dummyStartSignal {
					chStartAction <- nil
					startMonitor = true
				}
			}
		}
		select {
		case <-ctx.Done():
			// Timeout.
			if startMonitor {
				ch <- errors.New("failed due to timeout: couldn't find the start signal to start monitoring")
			} else {
				chStartAction <- errors.New("failed due to timeout: couldn't find the stop signal to stop monitoing")
			}
		default:
			if startMonitor {
				ch <- errors.New("failed while running dbus-monitor")
			} else {
				chStartAction <- errors.New("failed while running dbus-monitor")
			}
		}
	}()

	return stop, chStartAction
}

// inWhitelist returns true if the dbus call is one of the vaild expected calls.
func inWhitelist(str string) (bool, string) {
	whitelistDbusCmd := []string{insertUserProfile, popAllUserProfiles, createUserProfile, dummyEndSignal, dummyStartSignal}
	for _, cmd := range whitelistDbusCmd {
		if strings.Contains(str, cmd) {
			return true, cmd
		}
	}
	return false, ""
}

// assureMethodCalls assure that the expected methods are called.
func assureMethodCalls(ctx context.Context, expectedMethodCalls []string, calledMethods []string) error {
	if len(expectedMethodCalls) != len(calledMethods) {
		return errors.Errorf("found unexpected number of method calls: got %v, want %v ", calledMethods, expectedMethodCalls)
	}
	found := false
	for _, expected := range expectedMethodCalls {
		found = false
		for _, actual := range calledMethods {
			if expected == actual {
				found = true
				break
			}
		}
		if !found {
			return errors.Errorf("An expected method was not called: got %v, want %v ", calledMethods, expectedMethodCalls)
		}
	}

	return nil
}

// testStartShill tests all created path names during shill startup.
func testStartShill(ctx context.Context, env *testEnv) error {

	if err := startShill(ctx); err != nil {
		return err
	}

	if err := assureIsDir("/run/shill"); err != nil {
		return errors.Wrap(err, "failed asserting that /run/shill is a directory")
	}
	if err := assureIsDir("/var/lib/dhcpcd"); err != nil {
		return errors.Wrap(err, "failed asserting that /var/lib/dhcpcd is a directory")
	}
	if err := assurePathOwner("/var/lib/dhcpcd", "dhcp"); err != nil {
		return errors.Wrap(err, "failed asserting that the user owner of the path /run/lib/dhcpcd is dhcp")
	}
	if err := assurePathGroup("/var/lib/dhcpcd", "dhcp"); err != nil {
		return errors.Wrap(err, "failed asserting that the group owner of the path /run/lib/dhcpcd is dhcp")
	}
	if err := assureIsDir("/run/dhcpcd"); err != nil {
		return errors.Wrap(err, "failed asserting that /run/dhcpcd is a directory")
	}
	if err := assurePathOwner("/run/dhcpcd", "dhcp"); err != nil {
		return errors.Wrap(err, "failed asserting that the user owner of the path /run/dhcpcd is dhcp")
	}
	if err := assurePathGroup("/run/dhcpcd", "dhcp"); err != nil {
		return errors.Wrap(err, "failed asserting that the group owner of the path /run/dhcpcd is dhcp")
	}
	return nil
}

// testStartLoggedIn tests starting up shill while user is already logged in.
func testStartLoggedIn(ctx context.Context, env *testEnv) error {
	if err := os.Mkdir("/run/shill", os.ModePerm); err != nil {
		return errors.Wrap(err, "failed making the directory /run/shill")
	}

	if err := os.Mkdir("/run/shill/user_profiles", os.ModePerm); err != nil {
		return errors.Wrap(err, "failed making the directory /run/shill/user_profiles")
	}

	if err := createShillUserProfile("", env); err != nil {
		return errors.Wrap(err, "failed creating the shill user profile")
	}

	if err := os.Symlink(env.shillUserProfileDir, "/run/shill/user_profiles/chronos"); err != nil {
		return errors.Wrapf(err, "failed to Symlink %s to /run/shill/user_profiles/chronos", env.shillUserProfileDir)
	}

	if err := touch("/run/state/logged-in"); err != nil {
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

// testLogin tests the login process.
// Login should create a profile directory, then create and push
// a user profile, given no previous state.
func testLogin(ctx context.Context, env *testEnv) error {
	if err := startShill(ctx); err != nil {
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, dbusMonitorTimeout)
	defer cancel()

	stop, ch := dbusEventMonitor(timeoutCtx)
	if err := <-ch; err != nil {
		return err
	}

	if err := login(ctx, fakeUser); err != nil {
		return errors.Wrap(err, "failed logging in")
	}

	calledMethods, err := stop()
	if err != nil {
		return err
	}

	expectedCalls := []string{createUserProfile, insertUserProfile}
	if err := assureMethodCalls(ctx, expectedCalls, calledMethods); err != nil {
		return err
	}

	if err := assureExists(env.shillUserProfile); err != nil {
		return errors.Wrapf(err, "failed shill user profile %s doesn't exist", env.shillUserProfile)
	}

	if err := assureIsDir(env.shillUserProfileDir); err != nil {
		return errors.Wrapf(err, "failed asserting that %v is a directory", env.shillUserProfileDir)
	}

	if err := assureIsDir("/run/shill/user_profiles"); err != nil {
		return errors.Wrap(err, "failed asserting that /run/shill/user_profiles is a directory")
	}

	if err := assureIsLinkTo("/run/shill/user_profiles/chronos", env.shillUserProfileDir); err != nil {
		return err
	}

	if err := assureIsDir(env.userCryptohomeLogDir); err != nil {
		return errors.Wrapf(err, "failed asserting that %v is a directory", env.userCryptohomeLogDir)
	}

	if err := assureIsLinkTo("/run/shill/log", env.userCryptohomeLogDir); err != nil {
		return err
	}

	profiles, err := getProfileList(ctx)
	if err != nil {
		return err
	}

	if len(profiles) == 0 {
		return errors.Wrap(err, "profile list is empty")
	}

	// The last profile should be the one we just created.
	profilePath := string(profiles[len(profiles)-1])

	if profilePath != expectedProfileName {
		return errors.Wrapf(err, "found unexpected profile path: got %s, want %s", profilePath, expectedProfileName)
	}

	return nil
}

// testLoginGuest tests the guest login process.
// Login should create a temporary profile directory in /run,
// instead of using one within the root directory for normal users.
func testLoginGuest(ctx context.Context, env *testEnv) error {
	// Simulate guest login.
	// For guest login, session-manager passes an empty CHROMEOS_USER arg.
	if err := startShill(ctx); err != nil {
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, dbusMonitorTimeout)
	defer cancel()

	stop, ch := dbusEventMonitor(timeoutCtx)
	if err := <-ch; err != nil {
		return err
	}

	if err := login(ctx, ""); err != nil {
		return errors.Wrap(err, "failed logging in")
	}

	calledMethods, err := stop()
	if err != nil {
		return err
	}

	expectedCalls := []string{createUserProfile, insertUserProfile}
	if err := assureMethodCalls(ctx, expectedCalls, calledMethods); err != nil {
		return err
	}

	if err := assureNotExists(env.shillUserProfile); err != nil {
		return errors.Wrapf(err, "failed shill user profile %s does exist", env.shillUserProfile)
	}

	if err := assureNotExists(env.shillUserProfileDir); err != nil {
		return errors.Wrapf(err, "failed shill user profile directory %s does exist", env.shillUserProfileDir)
	}

	if err := assureIsDir(guestShillUserProfileDir); err != nil {
		return errors.Wrapf(err, "failed asserting that %v is a directory", guestShillUserProfileDir)
	}

	if err := assureIsDir("/run/shill/user_profiles"); err != nil {
		return errors.Wrap(err, "failed asserting that /run/shill/user_profiles is a directory")
	}

	if err := assureIsLinkTo("/run/shill/user_profiles/chronos", guestShillUserProfileDir); err != nil {
		return err
	}

	if err := assureIsDir(guestShillUserLogDir); err != nil {
		return errors.Wrapf(err, "failed asserting that %v is a directory", guestShillUserLogDir)
	}

	if err := assureIsLinkTo("/run/shill/log", guestShillUserLogDir); err != nil {
		return err
	}

	profiles, err := getProfileList(ctx)
	if err != nil {
		return err
	}

	if len(profiles) == 0 {
		return errors.Wrap(err, "profile list is empty")
	}

	// The last profile should be the one we just created.
	profilePath := string(profiles[len(profiles)-1])

	if profilePath != expectedProfileName {
		return errors.Wrapf(err, "found unexpected profile path: got %s, want %s", profilePath, expectedProfileName)
	}

	return nil
}

// testLoginProfileExists tests logging in a user whose profile already exists.
// Login script should only push (and not create) the user profile
// if a user profile already exists.
func testLoginProfileExists(ctx context.Context, env *testEnv) error {
	if err := startShill(ctx); err != nil {
		return err
	}

	if err := os.Mkdir(env.shillUserProfileDir, 0700); err != nil {
		return errors.Wrapf(err, "failed creating the directory: %s", env.shillUserProfileDir)
	}

	if err := testexec.CommandContext(ctx, "chown", "shill:shill", env.shillUserProfileDir).Run(); err != nil {
		return errors.Wrap(err, "failed changing the owner of the directory /run/shill/user_profiles to shill")
	}

	if err := os.Mkdir("/run/shill/user_profiles", 0700); err != nil {
		return errors.Wrap(err, "failed creating the directory: /run/shill/user_profiles")
	}

	if err := testexec.CommandContext(ctx, "chown", "shill:shill", "/run/shill/user_profiles").Run(); err != nil {
		return errors.Wrap(err, "failed changing the owner of the directory /run/shill/user_profiles to shill")
	}

	if err := os.Symlink(env.shillUserProfileDir, "/run/shill/user_profiles/chronos"); err != nil {
		return errors.Wrapf(err, "failed to Symlink %s to /run/shill/user_profiles/chronos", env.shillUserProfileDir)
	}

	if err := createProfile(ctx, chronosProfileName); err != nil {
		return err
	}

	if err := testexec.CommandContext(ctx, "rm", "-rf", "/run/shill/user_profiles/chronos").Run(); err != nil {
		return errors.Wrap(err, "failed removing the system state after it was archived")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, dbusMonitorTimeout)
	defer cancel()

	stop, ch := dbusEventMonitor(timeoutCtx)
	if err := <-ch; err != nil {
		return err
	}

	if err := login(ctx, fakeUser); err != nil {
		return errors.Wrap(err, "failed logging in")
	}

	calledMethods, err := stop()
	if err != nil {
		return err
	}

	expectedCalls := []string{insertUserProfile}
	if err := assureMethodCalls(ctx, expectedCalls, calledMethods); err != nil {
		return err
	}

	profiles, err := getProfileList(ctx)
	if err != nil {
		return err
	}

	if len(profiles) != 2 {
		return errors.Wrapf(err, "found unexpected number of profiles in the profile stack: got %d, want 2 ", len(profiles))
	}

	return nil
}

// testLogout tests the logout process.
func testLogout(ctx context.Context, env *testEnv) error {
	if err := startShill(ctx); err != nil {
		return err
	}

	if err := os.MkdirAll("/run/shill/user_profiles", 0777); err != nil {
		return errors.Wrap(err, "failed creating the directory: /run/shill/user_profiles")
	}

	if err := os.MkdirAll(guestShillUserProfileDir, 0777); err != nil {
		return errors.Wrapf(err, "failed creating the directory: %s", guestShillUserProfileDir)
	}

	if err := os.MkdirAll(guestShillUserLogDir, 0777); err != nil {
		return errors.Wrapf(err, "failed creating the directory: %s", guestShillUserLogDir)
	}

	if err := touch("/run/state/logged-in"); err != nil {
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, dbusMonitorTimeout)
	defer cancel()

	stop, ch := dbusEventMonitor(timeoutCtx)
	if err := <-ch; err != nil {
		return err
	}

	if err := logout(ctx); err != nil {
		return errors.Wrap(err, "failed logging out")
	}

	calledMethods, err := stop()
	if err != nil {
		return err
	}

	expectedCalls := []string{popAllUserProfiles}
	if err := assureMethodCalls(ctx, expectedCalls, calledMethods); err != nil {
		return err
	}

	if err := assureNotExists("/run/shill/user_profiles"); err != nil {
		return errors.Wrap(err, "failed shill user profile /run/shill/user_profiles does exist")
	}

	if err := assureNotExists(guestShillUserProfileDir); err != nil {
		return errors.Wrapf(err, "failed guest shill user profile directory %s does exist", guestShillUserProfileDir)
	}

	if err := assureNotExists(guestShillUserLogDir); err != nil {
		return errors.Wrapf(err, "failed guest shill user log directory %s does exist", guestShillUserLogDir)
	}

	profiles, err := getProfileList(ctx)
	if err != nil {
		return err
	}

	if len(profiles) > 1 {
		return errors.Wrapf(err, "found unexpected number of profiles in the profile stack: got %d, want 1 ", len(profiles))
	}

	return nil
}

// testLoginMultiProfile tests signalling shill about multiple logged-in users.
// Login script should not create multiple profiles in parallel
// if called more than once without an intervening logout.  Only
// the initial user profile should be created.
func testLoginMultiProfile(ctx context.Context, env *testEnv) error {
	if err := startShill(ctx); err != nil {
		return err
	}

	// First logged-in user should create a profile (tested above).
	if err := login(ctx, fakeUser); err != nil {
		return errors.Wrap(err, "failed logging in")
	}

	for i := 0; i < 5; i++ {
		timeoutCtx, cancel := context.WithTimeout(ctx, dbusMonitorTimeout)
		defer cancel()

		stop, ch := dbusEventMonitor(timeoutCtx)
		if err := <-ch; err != nil {
			return err
		}

		if err := login(ctx, fakeUser); err != nil {
			return errors.Wrap(err, "failed logging in")
		}

		calledMethods, err := stop()
		if err != nil {
			return err
		}

		var expectedCalls []string
		if err := assureMethodCalls(ctx, expectedCalls, calledMethods); err != nil {
			return err
		}

		files, err := ioutil.ReadDir("/run/shill/user_profiles")
		if err != nil {
			return err
		}
		if len(files) != 1 {
			return errors.Errorf("found unexpected number of profiles in the directorey /run/shill/user_profiles: got %v, want 1 ", len(files))
		}
		if files[0].Name() != "chronos" {
			return errors.Errorf("found unexpected profile link in the directorey /run/shill/user_profiles: got %v, want chronos ", files[0].Name())
		}
		if err := assureIsLinkTo("/run/shill/log", env.userCryptohomeLogDir); err != nil {
			return err
		}
	}

	profiles, err := getProfileList(ctx)
	if err != nil {
		return err
	}

	if len(profiles) != 2 {
		return errors.Wrapf(err, "found unexpected number of profiles in the profile dtack: got %d, want 2 ", len(profiles))
	}

	return nil
}

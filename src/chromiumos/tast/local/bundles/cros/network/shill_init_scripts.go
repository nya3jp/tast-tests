// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"syscall"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
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
	cryptohomePathCommand    = "/usr/sbin/cryptohome-path"
	guestShillUserProfileDir = "/run/shill/guest_user_profile/shill"
	guestShillUserLogDir     = "/run/shill/guest_user_profile/shill_logs"
	chronosProfileName       = "~chronos/shill"
	expectedProfileName      = "/profile/chronos/shill"
	dbusMonitorTimeout       = 5 * time.Second
	createUserProfile        = "CreateProfile"
	insertUserProfile        = "InsertUserProfile"
	popAllUserProfiles       = "PopAllUserProfiles"
	dummyEndSignal           = "DummyEndSignal"
)

type testEnv struct {
	rootCryptohomeDir    string
	userCryptohomeLogDir string
	shillUserProfileDir  string
	shillUserProfile     string
	createdDirectories   []string
}

// testFuncType takes a context.Context, testEnv struct, and return an error.
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

	for _, tc := range []struct {
		name string
		fn   testFuncType
	}{
		{"testStartShill", testStartShill},
		{"testStartLoggedIn", testStartLoggedIn},
		{"testLogin", testLogin},
		{"testLoginGuest", testLoginGuest},
		{"testLoginProfileExists", testLoginProfileExists},
		{"testLogout", testLogout},
		{"testLoginMultiProfile", testLoginMultiProfile},
	} {
		if err := runTest(ctx, &env, tc.name, tc.fn); err != nil {
			s.Fatalf("Failed running %s: %v", tc.name, err)
		}
	}
}

// runTest runs the test, stops shill and erases the state.
func runTest(ctx context.Context, env *testEnv, name string, fn testFuncType) error {
	if err := fn(ctx, env); err != nil {
		return err
	}
	if err := upstart.StopJob(ctx, "shill"); err != nil {
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
	env.createdDirectories = append(env.createdDirectories, "/var/cache/shill", "/run/shill", "/run/state/logged-in", "/run/dhcpcd", "/var/lib/dhcpcd")

	// Stop shill temporarily.
	if err := upstart.StopJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed stopping shill")
	}

	rootCryptDir, err := cryptohome.SystemPath(fakeUser)
	if err != nil {
		return errors.Wrap(err, "failed getting the cryptohome directory for the fake user")
	}

	env.rootCryptohomeDir = rootCryptDir

	// Deduce the directory for memory log storage.
	env.userCryptohomeLogDir = filepath.Join(env.rootCryptohomeDir, "shill_logs")

	// Just in case this hash actually exists, add these to the list of saved directories.
	env.createdDirectories = append(env.createdDirectories, env.rootCryptohomeDir)

	for _, dir := range env.createdDirectories {
		if err := os.RemoveAll(dir); err != nil {
			return errors.Wrapf(err, "failed removing %s while removing the system state", dir)
		}
	}

	// Create the fake user's system cryptohome directory.
	if err := os.Mkdir(env.rootCryptohomeDir, 0777); err != nil {
		return errors.Wrapf(err, "failed making the directory after removing the system state: %s", env.rootCryptohomeDir)
	}

	env.shillUserProfileDir = filepath.Join(env.rootCryptohomeDir, "shill")
	env.shillUserProfile = filepath.Join(env.shillUserProfileDir, "shill.profile")

	return nil
}

// tearDown performs cleanup at the end of the test.
func tearDown(ctx context.Context, env *testEnv) {
	var errMsg error
	// Stop any shill instances started during testing.
	if err := upstart.StopJob(ctx, "shill"); err != nil {
		errMsg = errors.Wrap(errMsg, errors.Wrap(err, "failed stopping shill").Error())
	}

	if err := eraseState(ctx, env); err != nil {
		errMsg = errors.Wrap(errMsg, errors.Wrap(err, "failed erasing the system state").Error())
	}

	if err := upstart.RestartJob(ctx, "shill"); err != nil {
		errMsg = errors.Wrap(errMsg, errors.Wrap(err, "failed restarting shill").Error())
	}

	testing.ContextLog(errMsg)
}

// eraseState removes all the test harness files.
func eraseState(ctx context.Context, env *testEnv) error {
	for _, dir := range env.createdDirectories {
		if err := os.RemoveAll(dir); err != nil {
			return errors.Wrapf(err, "failed removing %s while removing the system state", dir)
		}
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
		return nil, errors.Wrap(err, "failed creating shill manager object")
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
		return errors.Wrap(err, "failed creating shill manager object")
	}
	if _, err := manager.CreateProfile(ctx, chronosProfileName); err != nil {
		return errors.Wrapf(err, "failed creating profile: %v", chronosProfileName)
	}
	return nil
}

// dbusEventMonitor monitors the system message bus for those D-Bus calls we want to observe (insertUserProfile, popAllUserProfiles, createUserProfile).
// It returns a stop function and error. The stop function stops the D-Bus monitor and return the called methods and/or error.
func dbusEventMonitor(ctx context.Context) (func() ([]string, error), error) {
	ch := make(chan error, 1)
	var calledMethods []string
	stop := func() ([]string, error) {
		// Send a dummy dbus signal to stop the Eavesdrop.
		connect, err := dbus.SystemBus()
		if err != nil {
			return nil, errors.Wrap(err, "failed to connect to system bus")
		}
		if err := connect.Emit("/", fmt.Sprintf("com.dummy.%s", dummyEndSignal)); err != nil {
			return calledMethods, errors.Wrap(err, "failed sending dummy signal to stop Eavesdrop")
		}
		if err := <-ch; err != nil {
			return calledMethods, err
		}
		return calledMethods, nil
	}

	conn, err := dbus.SystemBusPrivate()
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to system bus")
	}
	err = conn.Auth(nil)
	if err != nil {
		conn.Close()
		return nil, errors.Wrap(err, "failed to authenticate the system bus")
	}
	err = conn.Hello()
	if err != nil {
		conn.Close()
		return nil, errors.Wrap(err, "failed to send the Hello call to the system bus")
	}

	var rules = []string{
		fmt.Sprintf("type='method_call',member='%s',path='/',interface='org.chromium.flimflam.Manager'", insertUserProfile),
		fmt.Sprintf("type='method_call',member='%s',path='/',interface='org.chromium.flimflam.Manager'", createUserProfile),
		fmt.Sprintf("type='method_call',member='%s',path='/',interface='org.chromium.flimflam.Manager'", popAllUserProfiles),
		fmt.Sprintf("type='signal',member='%s',path='/',interface='com.dummy'", dummyEndSignal),
	}

	call := conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.Monitoring.BecomeMonitor", 0, rules, uint(0))
	if call.Err != nil {
		return nil, errors.Wrap(call.Err, "failed to become monitor ")
	}

	c := make(chan *dbus.Message, 10)
	conn.Eavesdrop(c)

	go func() {
		defer func() {
			conn.Eavesdrop(nil)
			conn.Close()
		}()

		for {
			select {
			case <-ctx.Done():
				ch <- errors.New("failed waiting for signal")
			case msg := <-c:
				dbusCmd, err := dbusCallMember(msg)
				if err != nil {
					testing.ContextLog(ctx, "Something failed: ", err)
					continue
				}
				if dbusCmd == dummyEndSignal {
					ch <- nil
					return
				}
				calledMethods = append(calledMethods, dbusCmd)
			}
		}
	}()

	return stop, nil
}

// dbusCallMember returns the member name of the D-Bus call.
func dbusCallMember(dbusMessage *dbus.Message) (string, error) {
	v, ok := dbusMessage.Headers[dbus.FieldMember]
	if !ok {
		return "", errors.Errorf("failed dbus message doesn't have field member: %s", dbusMessage)
	}
	msg := fmt.Sprintf(v.String()[1 : len(v.String())-1])
	whitelistDbusCmd := []string{insertUserProfile, popAllUserProfiles, createUserProfile, dummyEndSignal}
	for _, cmd := range whitelistDbusCmd {
		if msg == cmd {
			return cmd, nil
		}
	}
	return "", errors.Errorf("failed found unexpected call: got %s, want %v", msg, whitelistDbusCmd)
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
	if err := upstart.StartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed starting shill")
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

	if err := upstart.StartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed starting shill")
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
	if err := upstart.StartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed starting shill")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, dbusMonitorTimeout)
	defer cancel()

	stop, err := dbusEventMonitor(timeoutCtx)
	if err != nil {
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
	if err := upstart.StartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed starting shill")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, dbusMonitorTimeout)
	defer cancel()

	stop, err := dbusEventMonitor(timeoutCtx)
	if err != nil {
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
	if err := upstart.StartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed starting shill")
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
	if err := os.RemoveAll("/run/shill/user_profiles/chronos"); err != nil {
		return errors.Wrap(err, "failed removing /run/shill/user_profiles/chronos")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, dbusMonitorTimeout)
	defer cancel()

	stop, err := dbusEventMonitor(timeoutCtx)
	if err != nil {
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
	if err := upstart.StartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed starting shill")
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

	stop, err := dbusEventMonitor(timeoutCtx)
	if err != nil {
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
	if err := upstart.StartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed starting shill")
	}

	// First logged-in user should create a profile (tested above).
	if err := login(ctx, fakeUser); err != nil {
		return errors.Wrap(err, "failed logging in")
	}

	for i := 0; i < 5; i++ {
		timeoutCtx, cancel := context.WithTimeout(ctx, dbusMonitorTimeout)
		defer cancel()

		stop, err := dbusEventMonitor(timeoutCtx)
		if err != nil {
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
		return errors.Wrapf(err, "found unexpected number of profiles in the profile stack: got %d, want 2 ", len(profiles))
	}

	return nil
}

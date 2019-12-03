// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shillscript has helper functions and the dbus monitor implementation for the shill init scripts tests.
package shillscript

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
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// FakeUser is used by the login function.
const (
	FakeUser                   = "not-a-real-user@chromium.org"
	CryptohomePathCommand      = "/usr/sbin/cryptohome-path"
	ShillUserProfilesDir       = "/run/shill/user_profiles"
	ShillUserProfileChronosDir = "/run/shill/user_profiles/chronos"
	GuestShillUserProfileDir   = "/run/shill/guest_user_profile/shill"
	GuestShillUserLogDir       = "/run/shill/guest_user_profile/shill_logs"
	ChronosProfileName         = "~chronos/shill"
	ExpectedProfileName        = "/profile/chronos/shill"
	DbusMonitorTimeout         = 5 * time.Second
	CreateUserProfile          = "CreateProfile"
	InsertUserProfile          = "InsertUserProfile"
	PopAllUserProfiles         = "PopAllUserProfiles"
	dummyEndSignal             = "DummyEndSignal"
)

// TestEnv struct has the variables that are used by the functions below.
type TestEnv struct {
	RootCryptohomeDir    string
	UserCryptohomeLogDir string
	ShillUserProfileDir  string
	ShillUserProfile     string
	CreatedDirectories   []string
}

// testFuncType takes a context.Context, TestEnv struct, and return an error.
type testFuncType func(ctx context.Context, env *TestEnv) error

// RunTest runs the test, stops shill and erases the state.
func RunTest(ctx context.Context, fn testFuncType) error {
	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		return errors.Wrap(err, "failed locking the check network hook")
	}
	defer unlock()

	var env TestEnv

	defer tearDown(ctx, &env)

	if err := setUp(ctx, &env); err != nil {
		return errors.Wrap(err, "failed starting the test")
	}

	return fn(ctx, &env)
}

// setUp setup the start of the test. Stop shill and create test harness.
func setUp(ctx context.Context, env *TestEnv) error {
	// The directories names that are created during the test and deleted at the end of the test.
	env.CreatedDirectories = append(env.CreatedDirectories, "/var/cache/shill", "/run/shill", "/run/state/logged-in", "/run/dhcpcd", "/var/lib/dhcpcd")

	// Stop shill temporarily.
	if err := upstart.StopJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed stopping shill")
	}

	rootCryptDir, err := cryptohome.SystemPath(FakeUser)
	if err != nil {
		return errors.Wrap(err, "failed getting the cryptohome directory for the fake user")
	}

	env.RootCryptohomeDir = rootCryptDir

	// Deduce the directory for memory log storage.
	env.UserCryptohomeLogDir = filepath.Join(env.RootCryptohomeDir, "shill_logs")

	// Just in case this hash actually exists, add these to the list of to-be-removed directories.
	env.CreatedDirectories = append(env.CreatedDirectories, env.RootCryptohomeDir)

	if err := eraseState(ctx, env); err != nil {
		testing.ContextLog(ctx, errors.Wrap(err, "failed erasing the system state"))
	}

	env.ShillUserProfileDir = filepath.Join(env.RootCryptohomeDir, "shill")
	env.ShillUserProfile = filepath.Join(env.ShillUserProfileDir, "shill.profile")

	return nil
}

// tearDown performs cleanup at the end of the test.
func tearDown(ctx context.Context, env *TestEnv) {
	// Stop any shill instances started during testing.
	if err := upstart.StopJob(ctx, "shill"); err != nil {
		testing.ContextLog(ctx, errors.Wrap(err, "failed stopping shill"))
	}

	if err := eraseState(ctx, env); err != nil {
		testing.ContextLog(ctx, errors.Wrap(err, "failed erasing the system state"))
	}

	if err := upstart.RestartJob(ctx, "shill"); err != nil {
		testing.ContextLog(ctx, errors.Wrap(err, "failed restarting shill"))
	}
}

// DbusEventMonitor monitors the system message bus for those D-Bus calls we want to observe (InsertUserProfile, PopAllUserProfiles, CreateUserProfile).
// It returns a stop function and error. The stop function stops the D-Bus monitor and return the called methods and/or error.
func DbusEventMonitor(ctx context.Context) (func() ([]string, error), error) {
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
		fmt.Sprintf("type='method_call',member='%s',path='/',interface='org.chromium.flimflam.Manager'", InsertUserProfile),
		fmt.Sprintf("type='method_call',member='%s',path='/',interface='org.chromium.flimflam.Manager'", CreateUserProfile),
		fmt.Sprintf("type='method_call',member='%s',path='/',interface='org.chromium.flimflam.Manager'", PopAllUserProfiles),
		fmt.Sprintf("type='signal',member='%s',path='/',interface='com.dummy'", dummyEndSignal),
	}

	call := conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.Monitoring.BecomeMonitor", 0, rules, uint(0))
	if call.Err != nil {
		return nil, errors.Wrap(call.Err, "failed to become monitor")
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
	whitelistDbusCmd := []string{InsertUserProfile, PopAllUserProfiles, CreateUserProfile, dummyEndSignal}
	for _, cmd := range whitelistDbusCmd {
		if msg == cmd {
			return cmd, nil
		}
	}
	return "", errors.Errorf("failed found unexpected call: got %s, want %v", msg, whitelistDbusCmd)
}

// AssureMethodCalls assure that the expected methods are called.
func AssureMethodCalls(ctx context.Context, expectedMethodCalls []string, calledMethods []string) error {
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

// eraseState removes all the test harness files.
func eraseState(ctx context.Context, env *TestEnv) error {
	for _, dir := range env.CreatedDirectories {
		if err := os.RemoveAll(dir); err != nil {
			testing.ContextLogf(ctx, "Failed removing %s while removing the system state", dir)
		}
	}
	if err := os.Mkdir(env.RootCryptohomeDir, 0777); err != nil {
		return errors.Wrapf(err, "failed making the directory after removing the system state: %s", env.RootCryptohomeDir)
	}
	return nil
}

// Login simulates the login process.
func Login(ctx context.Context, user string) error {
	chromeUser := "CHROMEOS_USER=" + user
	if err := upstart.StartJob(ctx, "shill-start-user-session", chromeUser); err != nil {
		return err
	}
	return nil
}

// Logout simulates the logout process.
func Logout(ctx context.Context) error {
	if err := upstart.StartJob(ctx, "shill-stop-user-session"); err != nil {
		return err
	}
	return nil
}

// CreateShillUserProfile creates a fake user profile with |contents|.
func CreateShillUserProfile(contents string, env *TestEnv) error {
	if err := os.Mkdir(env.ShillUserProfileDir, os.ModePerm); err != nil {
		return errors.Wrapf(err, "failed making the directory: %s", env.ShillUserProfileDir)
	}
	if err := CreateFileWithContents(env.ShillUserProfile, contents); err != nil {
		return errors.Wrapf(err, "failed creating the file: %s", env.ShillUserProfile)
	}
	return nil
}

// GetProfileList return the profiles in the profile stack.
func GetProfileList(ctx context.Context) ([]*shill.Profile, error) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed creating shill manager object")
	}
	// Refresh the in-memory profile list.
	if _, err := manager.GetProperties(ctx); err != nil {
		return nil, errors.Wrap(err, "failed refreshing the in-memeory profile list")
	}
	// Get current profiles.
	profiles, err := manager.Profiles(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed getting profile list")
	}
	return profiles, nil
}

// CreateProfile creates a new shill profile.
func CreateProfile(ctx context.Context, profileName string) error {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed creating shill manager object")
	}
	if _, err := manager.CreateProfile(ctx, ChronosProfileName); err != nil {
		return errors.Wrapf(err, "failed creating profile: %v", ChronosProfileName)
	}
	return nil
}

// AssureIsDir asserts that |path| is a directory.
func AssureIsDir(path string) error {
	if err := AssureExists(path); err != nil {
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

// AssureExists asserts that |path| exists.
func AssureExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return nil
}

// AssureNotExists asserts that |path| doesn't exist.
func AssureNotExists(path string) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	return errors.Errorf("%s exists unexpectedly", path)
}

// AssurePathOwner asserts that |path| is owned by |owner|.
func AssurePathOwner(path, owner string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return errors.Wrapf(err, "failed getting FileInfo interface of the path %s", path)
	}
	sys := stat.Sys().(*syscall.Stat_t)
	userID := fmt.Sprint(sys.Uid)
	userStruct, err := user.LookupId(userID)
	if err != nil {
		return errors.Wrapf(err, "failed getting the User struct from the user id %s of the path %s", userID, path)
	}
	if userName := userStruct.Username; userName != owner {
		return errors.Errorf("found unexpected (login name/ owner): got %s, want %s", userName, owner)
	}
	return nil
}

// AssurePathGroup asserts that |path| is owned by |group|.
func AssurePathGroup(path, group string) error {
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
	if groupName := groupStruct.Name; groupName != group {
		return errors.Errorf("found unexpected group name: got %s, want %s", groupName, group)
	}
	return nil
}

// AssureIsLinkTo asserts that |path| is a symbolic link to |pointee|.
func AssureIsLinkTo(path string, pointee string) error {
	if err := AssureIsLink(path); err != nil {
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

// AssureIsLink asserts that |path| is a symbolic link.
func AssureIsLink(path string) error {
	if err := AssureExists(path); err != nil {
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

// CreateFileWithContents creates a file named |filename| that contains |contents|.
func CreateFileWithContents(fileName, contents string) error {
	if err := ioutil.WriteFile(fileName, []byte(contents), 0644); err != nil {
		return err
	}
	return nil
}

// Touch creates an empty file named |filename|.
func Touch(filename string) error {
	if err := CreateFileWithContents(filename, ""); err != nil {
		return errors.Wrapf(err, "failed creating an empty file: %s", filename)
	}
	return nil
}

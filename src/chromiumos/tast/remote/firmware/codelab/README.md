# Tast FAFT Codelab: Remote Firmware Tests (go/tast-faft-codelab)

> This document assumes that you've already completed [A Tour of Go], [Codelab #1] and [Codelab #2].

[TOC]

This codelab follows the creation of a remote firmware test in Tast. In doing so, we'll learn how to do the following:

* Schedule the test to run during a FAFT suite
* Skip the test on DUTs that don't have a Chrome EC
* Collect information about the DUT via `firmware.Reporter`
* Read fw-testing-configs values via `firmware.Config`
* Send Servo commands
* Send RPC commands to the DUT
* Manage common firmware structures via `firmware.Helper`
* Boot the DUT into recovery/developer mode via `firmware.ModeSwitcher`
* Ensure that the DUT is in an expected state at the start and end of the test via `firmware.Pre`

In order to demonstrate what's happening "under the hood," some sections of this codelab will overwrite code from earlier sections. Thus, working through the codelab will teach you more than just studying the final code.

[A Tour of Go]: https://tour.golang.org/
[Codelab #1]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/codelab_1.md
[Codelab #2]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/codelab_2.md

## Boilerplate

Most firmware tests are remote tests, because they tend to disrupt the DUT, such as by rebooting it or corrupting its firmware.

Create a new remote test in the `firmware` bundle by creating a new file, `~/trunk/src/platform/tast-tests/src/chromiumos/tast/remote/bundles/cros/firmware/codelab.go`, with the following contents:

```go
// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Codelab,
		Desc: "Demonstrates common functionality for remote firmware tests",
		Contacts: []string{
			"me@chromium.org",      // Test author
			"my-team@chromium.org", // Backup mailing list
		},
		// TODO: Move to firmware_unstable, then firmware_ec
		Attr: []string{"group:firmware", "firmware_experimental"},
	})
}

func Codelab(ctx context.Context, s *testing.State) {
	s.Log("FAFT stands for Fully Automated Firmware Test")
}
```

Try running the test with the following command (inside the chroot). You'll need to replace `${HOST}` with your DUT's IP.

```
> tast run ${HOST} firmware.Codelab
```

You can find a copy of this code at [`codelab_basic.txt`].

> This directory contains many sample test files. Those test files have the `.txt` extension instead of the normal `.go` in order to avoid running in automated suites, and to avoid preupload errors for using AddTest in a support package. Those tests won't be able to run unless you move them into `tast-tests/src/chromiumos/tast/remote/bundles/cros/firmware`, and rename them with `.go` extensions.

[`codelab_basic.txt`]: ./codelab_basic.txt

## Attributes

Notice the `Attr` line in the above snippet. In previous Tast codelabs, we used the attributes `"group:mainline"` and `"informational"`. Those attributes cause tests to run in the CQ. However, most firmware tests are very expensive to run, and might not be appropriate to run in the CQ. You can read more about effective CQ usage on-corp at [go/effective-cq]. Additionally, most firmware tests should be run on the `faft-test` device pool, unlike the mainline tests.

For those reasons, firmware tests have a separate group of attributes. The group is called `"group:firmware"`, and has a handful of sub-attributes. You can find all of those sub-attributes in [attr.go], and you can learn more about how we use them to run FAFT tests at [go/faft-tast-via-tauto].

The `firmware_experimental` attribute is for tests that are particularly unstable. This mitigates the risk of accidentally putting a DUT into into a state that would cause other tests to fail. If we find that our test is stable enough, then we can promote it to another attribute, like `firmware_unstable` and eventually `firmware_ec` (or smoke, cr50, slow, ccd as appropriate). But for now, let's use `firmware_experimental`.

[attr.go]: https://chromium.googlesource.com/chromiumos/platform/tast/+/refs/heads/main/src/chromiumos/tast/internal/testing/attr.go
[go/effective-cq]: http://goto.google.com/effective-cq
[go/faft-tast-via-tauto]: http://goto.google.com/faft-tast-via-tauto

## Skip the test on DUTs without a Chrome EC

Many FAFT tests rely on certain hardware or software features. Per [go/tast-deps], the correct way to handle that in Tast is via HardwareDeps and SoftwareDeps.

Let's write a test that needs a Chrome EC. For context, some platforms have a Chrome EC (such as octopus), some platforms have a Wilco EC (such as sarien), and some platforms have no EC (such as rikku).

There is already a HardwareDep for ChromeEC, so let's use it.

We'll need to import `"chromiumos/tast/testing/hwdep"`, so add that to the imports:

```go
import (
	"context"

	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)
```

Then, we'll need to add a `HardwareDep` to the test definition:

```go
testing.AddTest(&testing.Test{
	...
	HardwareDeps: hwdep.D(hwdep.ChromeEC()),
})
```

Now, if you run your test on a DUT without a Chrome EC (such as rikku or sarien), it should skip without running.

For more information about HardwareDeps and SoftwareDeps, see [go/tast-deps]. If your test requires a dependency that isn't supported by Tast yet, make it! Others will thank you.

At this point (after running `gofmt`), your test file should resemble [`codelab_dependency.txt`].

[go/tast-deps]: http://goto.google.com/tast-deps
[`codelab_dependency.txt`]: ./codelab_dependency.txt

## Report DUT info

The remote `firmware` library has a utility structure called [`Reporter`], which has several methods for collecting useful firmware information from the DUT.

Let's use the `Reporter` to collect some basic information about the DUT, such as its board and model.

First, add the remote `firmware/reporters` library to your imports.

```go
import (
	"context"

	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)
```

Then, in the main body of your test, use [`reporters.New`] to initialize a Reporter object. (You can also remove that `s.Log` line about FAFT.)

```go
func Codelab(ctx context.Context, s *testing.State) {
	r := reporters.New(s.DUT())
```

Finally, use some `reporter` methods to collect information about the DUT.

```go
	board, err := r.Board(ctx)
	if err != nil {
		s.Fatal("Failed to report board: ", err)
	}
	model, err := r.Model(ctx)
	if err != nil {
		s.Fatal("Failed to report model: ", err)
	}
	s.Logf("Reported board=%s, model=%s", board, model)
}
```

Try running this test on your DUT. Did you get the results you expected?

At this point (after running `gofmt`), your test file should resemble [`codelab_reporter.txt`].

[`Reporter`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/firmware/reporters/reporter.go
[`reporters.New`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/firmware/reporters/reporter.go?q=New
[`codelab_reporter.txt`]: ./codelab_reporter.txt

## Read fw-testing-configs

fw-testing-configs are a set of JSON files defining platform-specific attributes for use in FAFT testing. You can read all about it at [go/cros-fw-testing-configs-guide]. Config data for all platforms gets consolidated into a single data file called `CONSOLIDATED.json`.

In Tast, we access that consolidated JSON as a [data file]. The relative path to that data file is exported in the remote `firmware` library as [`firmware.ConfigFile`].

To use that data file in our test, we first have to import the remote `firmware` library, and declare that our test uses the data file:

```go
import (
	"context"

	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		...
		Data: []string{firmware.ConfigFile},
	})
}
```

The [`firmware.NewConfig`] constructor requires three parameters: the full path to the data file, and the DUT's board and model. The full path to the data file can be acquired via [`s.DataPath`]. Thanks to the previous section, we already have the board and model.

```go
func Codelab(ctx context.Context, s *testing.State) {
	...
	cfg, err := firmware.NewConfig(s.DataPath(firmware.ConfigFile), board, model)
	if err != nil {
		s.Fatal("Failed to create config: ", err)
	}
```

Finally, we can access the config data via the `Config` struct's fields. If the field you want to reference isn't yet included in the `Config` struct, go ahead and add it.

```go
	s.Log("This DUT's mode-switcher type is: ", cfg.ModeSwitcherType)
}
```

At this point (after running `gofmt`), your test file should resemble [`codelab_config.txt`].

[go/cros-fw-testing-configs-guide]: https://chromium.googlesource.com/chromiumos/platform/fw-testing-configs/#cros-fw_testing_configs_user_s-guide
[data file]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/writing_tests.md#Data-files
[`firmware.ConfigFile`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/firmware/config.go?q=ConfigFile
[`firmware.NewConfig`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/firmware/config.go?q=%22func%20NewConfig%22
[`s.DataPath`]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/writing_tests.md#Data-files
[`codelab_config.txt`]: ./codelab_config.txt

## Servo

Many firmware tests rely on [Servo] for controlling the DUT. Let's use Servo in our test.

In order to send commands via Servo, the test needs to know the address of the machine running servod (the "servo\_host"), and the port on which that machine is running servod (the "servo\_port"). These values are supplied at runtime as a [runtime variable], of the form `${SERVO_HOST}:${SERVO_PORT}`.

To start, we will need to declare `"servo"` as a variable in the test:

```go
func init() {
	testing.AddTest(&testing.Test{
		...
		Vars: []string{"servo"},
	})
}
```

Next, in the test body, we will need to create a `servo.Proxy` object, which forwards commands to servod. The [`NewProxy`] constructor requires the servo host:port, and a keyFile and keyDir that can be obtained via the test's `DUT` object. Additionally, we should close the Proxy at the end of the test (via `defer`).

First, import the remote servo library:

```go
import (
	...
	"chromiumos/tast/remote/servo"
)
```

Append the following to the test body:

```go
func Codelab(ctx context.Context, s *testing.State) {
	...
	// Set up Servo
	dut := s.DUT()
	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)
}
```

Let's use Servo to find out the DUT's `ec_board`. `ec_board` is a simple GPIO control returning a string. We can get the value of a string control via the Servo method [`GetString`], defined in [`methods.go`]. That method takes a parameter of the type `StringControl`. `methods.go` defines a bunch of different `StringControl`s, including one called `ECBoard` (with value `ec_board`).

We'll have to extract the `Servo` object from our `Proxy`, and then call its `GetString` method with `servo.ECBoard` as the control parameter. Add the following to the test body:

```go
func Codelab(ctx context.Context, s *testing.State) {
	...
	// Get the DUT's ec_board via Servo
	ecBoard, err := pxy.Servo().GetString(ctx, servo.ECBoard)
	if err != nil {
		s.Fatal("Getting ec_board control from servo: ", err)
	}
	s.Log("EC Board: ", ecBoard)
}
```

`methods.go` defines a lot of Servo commands, but not nearly all of them. If you want to use a command that isn't represented in `methods.go`, go ahead and add it!

Note that `methods.go` takes advantage of Go's type system to define which values can be sent to certain controls. For example, Servo supports several controls representing keypresses, such as `ctrl_enter` and `power_key`, which each accept a duration-type string value: `"press"`, `"long_press"`, or `"tab"`. In `methods.go`, these controls are given the type [`KeypressControl`], and their acceptable values are given the type [`KeypressDuration`]. This allows tests to call the Servo method [`KeypressWithDuration`], such as `pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.LongPress)`. This reduces the chance of inadvertently sending an invalid string, and makes it easy for future developers to understand what acceptable values are for each control.

Try running your test using the same syntax as in previous sections:

```
(inside) > tast run ${HOST} firmware.Codelab
```

What happened? Your test failed, because you didn't supply the command-line variable `servo`, which our code referred to as a `RequiredVar`. So, treating ${SERVO\_HOST} and ${SERVO\_PORT} as your servo host and servo port respectively, try the following command:

```
(inside) > tast run -var=servo=${SERVO_HOST}:${SERVO_PORT} $HOST firmware.Codelab
```

What happened? If the servo host machine was running `servod` on the servo port, then your test probably ran successfully. Otherwise, you probably saw the following error message:

```
Error at codelab.txt:54: Failed to create servo: Post "http://127.0.0.1:42529": read tcp 127.0.0.1:60326->127.0.0.1:42529: read: connection reset by peer
```

You'll need to SSH into the servo host machine (if it's different from your workstation) and run `servod` (such as via `start servod PORT=${SERVO_PORT}`). Then try your `tast run` command again. Did it work? It should have.

For reference on running tests with Servo, you can review the [relevant section] of [go/tast-running].

At this point (after running `gofmt`), your test file should resemble [`codelab_servo.txt`].

[Servo]: https://chromium.googlesource.com/chromiumos/third_party/hdctools/+/HEAD/docs/servo.md
[runtime variable]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/writing_tests.md#Runtime-variables
[`NewProxy`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/servo/proxy.go?q=NewProxy
[`GetString`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/servo/methods.go?q=func.*GetString
[`methods.go`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/servo/methods.go
[`KeypressControl`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/servo/methods.go?q=%22type%20KeypressControl%22
[`KeypressDuration`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/servo/methods.go?q=%22type%20KeypressDuration%22
[`KeypressWithDuration`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/servo/methods.go?q=KeypressWithDuration
[relevant section]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/running_tests.md#running-tests-with-servo
[go/tast-running]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/running_tests.md
[`codelab_servo.txt`]: ./codelab_servo.txt

## RPC

Many firmware tests need to perform complicated subroutines on the DUT. Rather than calling many individual SSH commands, it is faster and stabler to send a single command via RPC. Fortunately, Tast has [built-in gRPC support].

Let's use the [BIOS service] to get the DUT's current GBB flags.

We'll need to add three imports: Tast's `rpc` library, the Tast firmware service library, and a library called `empty` (which we use for sending RPC requests containing no data). Add these to the file's imports:

```go
import (
	...
	"github.com/golang/protobuf/ptypes/empty"

	...
	fwService "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/rpc"
)
```

Note that we have imported the firmware service library under the alias `fwService`. This is to avoid a namespace collision with the remote firmware library (`"chromiumos/tast/remote/firmware"`)—otherwise, both would be called `firmware`.

Next, declare the BIOS service as a ServiceDep in the test's initialization:

```go
func init() {
	testing.AddTest(&testing.Test{
		...
		ServiceDeps: []string{"tast.cros.firmware.BiosService"},
	}
```

In the test body, initialize an RPC connection:

```go
func Codelab(ctx context.Context, s *testing.State) {
	...
	// Connect to RPC
	cl, err := rpc.Dial(ctx, dut, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)
```

Create a BIOS service client, which we will use to call BIOS-related RPCs:

```go
	bios := fwService.NewBiosServiceClient(cl.Conn)
```

Finally, call the `GetGBBFlags` RPC, and report results:

```go
	// Get current GBB flags via RPC
	flags, err := bios.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get GBB flags: ", err)
	}
	s.Log("Clear GBB flags: ", flags.Clear)
	s.Log("Set GBB flags:   ", flags.Set)
}
```

At this point (after running `gofmt`), your test file should resemble [`codelab_rpc.txt`].

[built-in gRPC support]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/writing_tests.md#Remote-procedure-calls-with-gRPC
[BIOS service]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/services/cros/firmware/bios_service.proto
[`codelab_rpc.txt`]: ./codelab_rpc.txt

## Simplify with Helper

In the above sections, we wrote 25 lines of code just to initialize a Servo, Config, and RPC client—not to mention additional code to actually _use_ those structures. If we had to include all that boilerplate in every firmware test, it would violate the [DRY principle].

For that reason, we have a structure called [`firmware.Helper`], whose job is to manage other remote firmware structures. Let's simplify our test using a `Helper`.

At the start of your test body, initialize a `firmware.Helper`. The [`NewHelper`] constructor requires several parameters, which it will use later to initialize other structures: `dut` (to construct the Reporter and Servo), `rpcHint` (for the RPC connection), `cfgFilepath` (for the Config), and `servoHostPort` (for Servo).

```go
func Codelab(ctx context.Context, s *testing.State) {
	servoSpec, _ := s.Var("servo")
	h := firmware.NewHelper(s.DUT(), s.RPCHint(), s.DataPath(firmware.ConfigFile), servoSpec)
	defer h.Close(ctx)
	...
}
```

The Helper now has all the information it needs to create a Reporter, Servo, Config, and RPC connection. Additionally, `h.Close` will close any firmware structures that it initialized.

The Helper constructs a Reporter during `NewHelper`, using the `DUT` that we passed in, so we can use that right away. Replace the `reporters.New` constructor in your test:

```go
	// OLD
	r := reporters.New(s.DUT())

	// NEW
	r := h.Reporter
```

If you prefer, you can use `h.Reporter` directly, without binding to a new variable:

```go
	board, err := h.Reporter.Board(ctx)
```

But for today, we'll leave it as `r`.

The other constructors are a little more complicated. `Helper` is lazy about initializing most structures, so that it can avoid unnecessary operations. For example, if a test doesn't require `Config`, then there is no need to spend time fetching the DUT's board and model. But we do want to use a `Config`, so let's create one.

The convention for such constructing a `Foo` via `Helper` is `h.RequireFoo()`. If the `Helper` is already managing a `Foo`, then it won't create a new one; thus, rather than specifying that we need a _new_ Foo, we _require_ that one exist.

So, let's replace the `Config` constructor in our test with `h.RequireConfig`.

```go
	// OLD
	cfg, err := firmware.NewConfig(s.DataPath(firmware.ConfigFile), board, model)
	if err != nil {
		s.Fatal("Failed to create config: ", err)
	}
	s.Log("This DUT's mode-switcher type is: ", cfg.ModeSwitcherType)

	// NEW
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to create config: ", err)
	}
	s.Log("This DUT's mode-switcher type is: ", h.Config.ModeSwitcherType)
```

Note that we didn't need to pass the board and model to `RequireConfig`; it fetched them via its `Reporter`. And, note that `RequireConfig` didn't return a `Config` object; it was stored as `h.Config`.

Next, let's use our `Helper` to create a Servo.

```go
	// OLD

	// Set up Servo
	dut := s.DUT()
	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	// Get the DUT's ec_board via Servo
	ecBoard, err := pxy.Servo().GetString(ctx, servo.ECBoard)
	if err != nil {
		s.Fatal("Getting ec_board control from servo: ", err)
	}
	s.Log("EC Board: ", ecBoard)

	// NEW

	// Get the DUT's ec_board via Servo
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	ecBoard, err := h.Servo.GetString(ctx, servo.ECBoard)
	if err != nil {
		s.Fatal("Getting ec_board control from servo: ", err)
	}
	s.Log("EC Board: ", ecBoard)
```

As described above, note that we don't need to defer `h.Servo.Close`. That will be called by `h.Close`, which we have already deferred.

Finally, let's use our `Helper` to initialize the RPC connection and BIOS service client.

```go
	// OLD

	// Connect to RPC
	cl, err := rpc.Dial(ctx, dut, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Get current GBB flags via RPC
	bios := fwService.NewBiosServiceClient(cl.Conn)
	flags, err := bios.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get GBB flags: ", err)
	}

	// NEW

	// Get current GBB flags via RPC
	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Failed to connect to RPC service on the DUT: ", err)
	}
	flags, err := h.BiosServiceClient.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get GBB flags: ", err)
	}
```

Notice that we didn't have to dial an RPC connection before creating the BIOS service client. That's because `h.RequireBiosServiceClient` calls another `Require` method in its implementation, `h.RequireRPCClient`. As we will see in the next section, some `Helper` constructors make heavy use of nested requirements like this.

If you try to run your code, the compiler will throw unused-import errors, due to our removed code. You can go ahead and delete any unused imports (`fwService`, `reporters`, and `rpc`).

At this point (after running `gofmt`), your test file should resemble [`codelab_helper.txt`].

[DRY principle]: https://en.wikipedia.org/wiki/Don%27t_repeat_yourself
[`firmware.Helper`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/firmware/helper.go?q=f:helper.go%20firmware%20tast-tests
[`NewHelper`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/firmware/helper.go;l=81?q=func.*NewHelper&sq=
[`codelab_helper.txt`]: ./codelab_helper.txt

## Switch the boot-mode

Let's reboot the DUT into recovery mode.

There is a structure called a [`ModeSwitcher`], which can boot the DUT into normal mode, recovery mode, and developer mode. It can also perform a mode-aware reset, which resets the DUT while retaining the boot-mode. The [`NewModeSwitcher`] constructor requires a `Helper`, because switching boot-modes requires a Config, a Servo, and an RPC connection.

Append the following to the test body to create a `ModeSwitcher`:

```go
func Codelab(ctx context.Context, s *testing.State) {
	...
	// Switch to recovery mode
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Failed to create mode-switcher: ", err)
	}
```

Then use the `ModeSwitcher` to switch to recovery mode. The constants for different boot-modes are defined in Tast's [`common/firmware`] library, which allows us to access them from both local and remote tests. Let's add that to the imports:

```go
import (
	...
	fwCommon "chromiumos/tast/common/firmware"
)
```

Finally, in the main test body, use the `ModeSwitcher` to reboot to recovery mode.

```go
	if err := ms.RebootToMode(ctx, fwCommon.BootModeRecovery); err != nil {
		s.Fatal("Failed to boot to recovery mode: ", err)
	}
}
```

We don't need to verify the DUT's boot mode after rebooting; `ms.RebootToMode` does that, and returns an error if the DUT ends up in an unexpected boot-mode.

If you try running this test, it will fail due to an undeclared service dependency. `RebootToMode` uses an RPC service, `tast.cros.firmware.UtilsService`, which we didn't declare in `ServiceDeps`. So, update the `ServiceDeps` line in the test initialization:

```go
func init() {
	testing.AddTest(&testing.Test{
		...
		ServiceDeps: []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		...
	})
}
```

As it stands, this test does something rude: it leaves the DUT in recovery mode. When the next test starts, the DUT will still be in recovery mode, which could cause unexpected behavior. This is bad! You could clean up manually by rebooting back to normal mode. But in the next section, we'll explore a more defensive alternative.

At this point (after running `gofmt`), your test file should resemble [`codelab_boot_mode.txt`].

[`ModeSwitcher`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/firmware/boot_mode.go?q=%22type%20ModeSwitcher%22
[`NewModeSwitcher`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/firmware/boot_mode.go?q=%22func%20NewModeSwitcher%22
[`common/firmware`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/common/firmware/
[`codelab_boot_mode.txt`]: ./codelab_boot_mode.txt

## Control start/end state with firmware.Pre

> TODO (gredelston): After b/174846911, update this section to describe Fixture instead of Pre. For context, [Fixtures] are an updated version of Precondition, but they do not yet support data files, which are required for our use-case.

Tast has a wonderful feature called [Preconditions]. This allows us to perform certain actions before and after each test. If several tests have the same Precondition, they will all be run in a row.

This is really useful for firmware testing. In FAFT, we like to ensure that the GBB flags start and end in an expected state. We also have many tests that have to run in normal mode, many others that run in recovery mode, and others that run in developer mode. Clumping those tests together means that we can boot into recovery mode once, and then run all of the recovery mode tests. It also makes cleanup easier, because if a DUT ends the test in an unexpected state (such as a strange boot mode or strange GBB flags), the Precondition will return it to the expected state.

Let's add a Precondition to our test to ensure that it starts and ends in normal mode. First, import the [`remote/firmware/pre`] library:

```go
import (
	...
	"chromiumos/tast/remote/firmware/pre"
)
```

In the test initialization, declare a `Pre` of `pre.NormalMode()`:

```go
func init() {
	testing.AddTest(&testing.Test{
		...
		Data:         []string{firmware.ConfigFile},
		Pre:          pre.NormalMode(),
		ServiceDeps:  []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
                SoftwareDeps: []string{"crossystem", "flashrom"},
		Vars:         []string{"servo"},
	})
}
```

Now, before the test runs, the test harness will invoke the precondition's `Prepare` method, which will put it into normal-mode. After all tests using this same precondition have finished, the test harness will invoke its `Close` method, which restores the DUT's GBB flags and boot-mode from before the tests began.

The `Pre` has a built-in `Helper`, so we don't need to create our own. Let's replace the `NewHelper` line so that we can reuse the `Pre`'s `Helper`.

```go
	// OLD
	servoSpec, _ := s.Var("servo")
	h := firmware.NewHelper(s.DUT(), s.RPCHint(), s.DataPath(firmware.ConfigFile), servoSpec)
	defer h.Close(ctx)

	// NEW
	h := s.PreValue().(*pre.Value).Helper
```

Note that we don't need to close the `Helper`, because the `Pre` will use it again at the end of all tests, and will close it afterward.

Go ahead and run your code—this is the last time we'll modify it.

At this point (after running `gofmt`), your test file should resemble [`codelab_pre.txt`].

[Fixtures]: http://doc/1kA79M7bB4O0tje-sdOuX6BIL3YmC8eyEqkwNaKvMEJI#heading=h.5irk4csrpu0y
[Preconditions]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/writing_tests.md#Preconditions
[`remote/firmware/pre`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/firmware/pre/
[`codelab_pre.txt`]: ./codelab_pre.txt

## Reviews

When you write firmware-related CLs in Tast, please follow the process prescribed at [go/tast-reviews]. Your CL should be reviewed by a test-owner and by a Tast-owner: that is, someone with subject-matter expertise, and someone with harness expertise.

There is a [gwsq] alias for Tast firmware-library reviews: tast-fw-library-reviewers@google.com. If you set that alias as a reviewer in Gerrit, it will be re-assigned to somebody with domain expertise. If you're not sure who should review your code, that's a great place to start. If you'd like to join that group of reviewers (which is a great way to learn more about FAFT), please email cros-fw-engrod@google.com.

This concludes the FAFT-in-Tast codelab. Congratulations! We look forward to reviewing your CLs.

[go/tast-reviews]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/code_reviews.md
[gwsq]: http://g3doc/gws/tools/gwsq/v3/g3doc/README

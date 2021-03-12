# Tast FAFT Codelab: Remote Firmware Tests

> This document assumes that you've already completed [A Tour of Go], [Codelab #1] and [Codelab #2].

This codelab follows the creation of a remote firmware test in Tast. In doing so, we'll learn how to do the following:

* Schedule the test to run during a FAFT suite
* Skip the test on DUTs that don't have a Chrome EC
* Collect information about the DUT via `firmware.Reporter`
* Read fw-testing-configs values via `firmware.Config`
* Send Servo commands
* Send RPC commands to the DUT
* Manage common firmware structures via `firmware.Helper`
* Boot the DUT into recovery/developer mode via `firmware.ModeSwitcher`
* Verify the DUT state at the start and end of the test via `firmware.Pre`

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
			"me@chromium.org",     // Test author
			"my-team@chromium.org, // Backup mailing list
		},
		Attr: []string{"group:firmware", "firmware_experimental"},
	})
}

func Codelab(ctx context.Context, s *testing.State) {
	s.Log("FAFT stands for Fully Automated Firmware Test!")
}
```

Try running the test with the following command (inside the chroot). You'll need to replace `$HOST` with your DUT's IP.

```
> tast run $HOST firmware.Codelab
```

## Attributes

Notice the `Attr` line in the above snippet. In previous Tast codelabs, we used the attributes `"group:mainline"` and `"informational"`. Those attributes cause tests to run in the CQ. However, most firmware tests are very expensive to run, and might not be appropriate to run in the CQ. You can read more about effective CQ usage on-corp at [go/effective-cq]. Additionally, most firmware tests should be run on the `faft-test` device pool, unlike the mainline tests.

For those reasons, firmware tests have a separate group of attributes. The group is called `"group:firmware"`, and has a handful of sub-attributes. You can find all of those sub-attributes in [attr.go], and you can learn more about how we use them to run FAFT tests at [go/faft-tast-via-tauto].

The `firmware_experimental` attribute is for tests that are particularly unstable. This mitigates the risk of accidentally putting a DUT into into a state that would cause other tests to fail. If we find that our test is stable enough, then we can promote it to another attribute, like `firmware_smoke`. But for now, let's use `firmware_experimental`.

[attr.go]: https://chromium.googlesource.com/chromiumos/platform/tast/+/refs/heads/main/src/chromiumos/tast/internal/testing/attr.go
[go/effective-cq]: http://goto.google.com/effective-cq
[go/faft-tast-via-tauto]: http://goto.google.com/faft-tast-via-tauto

## Skip the test on DUTs without a Chrome EC

Lots of FAFT tests rely on certain hardware features. Per [go/tast-deps], the correct way to handle that in Tast is via HardwareDeps and SoftwareDeps.

For today, let's write a test that needs a Chrome EC. For context, some DUTs have a Chrome EC (such as octopus), some DUTs have a Wilco EC (such as sarien), and some DUTs have no EC (such as rikku).

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

At this point (after running `gofmt`), your test file should look like the following:

```go
// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Codelab,
		Desc: "Demonstrates common functionality for remote firmware tests",
		Contacts: []string{
			"me@chromium.org",     // Test author
			"my-team@chromium.org, // Backup mailing list
		},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func Codelab(ctx context.Context, s *testing.State) {
	s.Log("FAFT stands for Fully Automated Firmware Test!")
}
```

[go/tast-deps]: http://goto.google.com/tast-deps

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

Then, in the main body of your test, use [`reporters.New`] to initialize a Reporters object. (You can also remove that `s.Log` line about FAFT.)

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

At this point (after running `gofmt`), your test file should look like the following:

```go
// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Codelab,
		Desc: "Demonstrates common functionality for remote firmware tests",
		Contacts: []string{
			"me@chromium.org",     // Test author
			"my-team@chromium.org, // Backup mailing list
		},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func Codelab(ctx context.Context, s *testing.State) {
	r := reporters.New(s.DUT())

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

[`Reporter`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/firmware/reporters/reporter.go
[`reporters.New`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/firmware/reporters/reporter.go?q=New

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
		Data: []string{firmware.ConfigFile}
	})
}
```

The [`firmware.NewConfig`] constructor requires three parameters: the full path to the data file, and the DUT's board and model. The full path to the data file can be acquired via [`s.DataPath`]. Thanks to the previous section, we already have the board and model.

```go
func Codelab(ctx context.Context, s *testing.State) {
	...
	cfg, err := firmware.NewConfig(s.DataPath(firmware.ConfigFile), board, model)
	if err != nil {
		s.Fatal("Failed to create config file: ", err)
	}
```

Finally, we can access the config data via the `Config` struct's fields. If the field you want to reference isn't yet included in the `Config` struct, go ahead and add it.

```go
	s.Log("This DUT's mode-switcher type is: ", cfg.ModeSwitcherType)
}
```

At this point (after running `gofmt`), your test file should look like the following:

```go
// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Codelab,
		Desc: "Demonstrates common functionality for remote firmware tests",
		Contacts: []string{
			"me@chromium.org",     // Test author
			"my-team@chromium.org, // Backup mailing list
		},
		Data:         []string{firmware.ConfigFile}
		Attr:         []string{"group:firmware", "firmware_experimental"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func Codelab(ctx context.Context, s *testing.State) {
	r := reporters.New(s.DUT())
	board, err := r.Board(ctx)
	if err != nil {
		s.Fatal("Failed to report board: ", err)
	}
	model, err := r.Model(ctx)
	if err != nil {
		s.Fatal("Failed to report model: ", err)
	}
	s.Logf("Reported board=%s, model=%s", board, model)

	cfg, err := firmware.NewConfig(s.DataPath(firmware.ConfigFile), board, model)
	if err != nil {
		s.Fatal("Failed to create config file: ", err)
	}
	s.Log("This DUT's mode-switcher type is: ", cfg.ModeSwitcherType)
}
```

[go/cros-fw-testing-configs-guide]: https://chromium.googlesource.com/chromiumos/platform/fw-testing-configs/#cros-fw_testing_configs_user_s-guide
[data file]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/writing_tests.md#Data-files
[`firmware.ConfigFile`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/firmware/config.go?q=ConfigFile
[`config.go`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/firmware/config.go
[`firmware.NewConfig`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/firmware/config.go?q=%22func%20NewConfig%22
[`s.DataPath`]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/writing_tests.md#Data-files

## Servo

Many firmware tests rely on [Servo] for controlling the DUT. Let's use Servo in our test.

In order to send commands via Servo, the test needs to know the address of the machine running servod (the "servo\_host"), and the port on which that machine is running servod (the "servo\_port"). These values are supplied at runtime as a [runtime variable], of the form `${SERVO_HOST}:${SERVO_PORT}`.

To start, we will need to declare `"servo"` as a variable in the test:

```go
func init() {
	testing.AddTest(&testing.Test{
		...
		Var: []string{"servo"},
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
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)
}
```

Servo methods are defined in [`methods.go`]. Let's use Servo to find out the DUT's `ec_board`. `ec_board` is a simple GPIO control returning a string; in `methods.go`, it's represented by a `StringControl` called `ECBoard`. There's a method on `servo.Servo` for getting the value of a `StringControl`: [`GetString`].

We'll have to extract the `Servo` object from our `Proxy`, and then call its `GetString` method with `servo.ECBoard` as the control parameter. Add the following to the test body:

```go
func Codelab(ctx context.Context, s *testing.State) {
	...
	// Get the DUT's ec_board via Servo
	ecBoard, err := pxy.Servo().GetString(ctx, servo.ECBoard)
	if err != nil {
		s.Fatal("getting ec_board control from servo:", err)
	}
	s.Log("EC Board: ", ecBoard)
}
```

`methods.go` defines a lot of Servo commands, but not nearly all of them. If you want to use a command that isn't represented in `methods.go`, go ahead and add it!

Note that `methods.go` takes advantage of Go's type system to define which values can be sent to certain controls. For example, Servo supports several controls representing keypresses, such as `ctrl_enter` and `power_key`, which each accept a duration-type string value: `"press"`, `"long_press"`, or `"tab"`. In `methods.go`, these controls are given the type [`KeypressControl`], and their acceptable values are given the type [`KeypressDuration`]. This allows tests to call the Servo method [`KeypressWithDuration`], such as `pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.LongPress)`. This reduces the chance of inadvertently sending an invalid string, and makes it easy for future developers to understand what acceptable values are for each control.

At this point (after running `gofmt`), your test file should look like the following:

```go
// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Codelab,
		Desc: "Demonstrates common functionality for remote firmware tests",
		Contacts: []string{
			"me@chromium.org",     // Test author
			"my-team@chromium.org, // Backup mailing list
		},
		Data:         []string{firmware.ConfigFile}
		Attr:         []string{"group:firmware", "firmware_experimental"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Var:          []string{"servo"},
	})
}

func Codelab(ctx context.Context, s *testing.State) {
	r := reporters.New(s.DUT())
	board, err := r.Board(ctx)
	if err != nil {
		s.Fatal("Failed to report board: ", err)
	}
	model, err := r.Model(ctx)
	if err != nil {
		s.Fatal("Failed to report model: ", err)
	}
	s.Logf("Reported board=%s, model=%s", board, model)

	cfg, err := firmware.NewConfig(s.DataPath(firmware.ConfigFile), board, model)
	if err != nil {
		s.Fatal("Failed to create config file: ", err)
	}
	s.Log("This DUT's mode-switcher type is: ", cfg.ModeSwitcherType)

	// Set up Servo
	dut := s.DUT()
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	// Get the DUT's ec_board via Servo
	ecBoard, err := pxy.Servo().GetString(ctx, servo.ECBoard)
	if err != nil {
		s.Fatal("getting ec_board control from servo:", err)
	}
	s.Log("EC Board: ", ecBoard)
}
```

[Servo]: https://chromium.googlesource.com/chromiumos/third_party/hdctools/+/HEAD/docs/servo.md
[runtime variable]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/writing_tests.md#Runtime-variables
[`NewProxy`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/servo/proxy.go?q=NewProxy
[`methods.go`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/servo/methods.go
[`GetString`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/servo/methods.go?q=func.*GetString
[`KeypressControl`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/servo/methods.go?q=%22type%20KeypressControl%22
[`KeypressDuration`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/servo/methods.go?q=%22type%20KeypressDuration%22
[`KeypressWithDuration`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/servo/methods.go?q=KeypressWithDuration

## RPC

Many firmware tests need to perform complicated subroutines on the DUT. Rather than calling many individual SSH commands, it is faster and stabler to send a single command via RPC. Fortunately, Tast has [built-in gRPC support].

Let's use the [BIOS service] to get the DUT's current GBB flags.

We'll need to add three imports: Tast's `rpc` library, the Tast firmware service library, and a library called `empty` (which we use for sending RPC requests containing no data). Add these to the file's imports:

```go
import (
	...
	"github.com/golang/protobuf/ptypes/empty"

	...
	fwService "chromiumos/tast/services/firmware"
	"chromiumos/tast/rpc"
)
```

Note that we have imported the firmware service library under the alias `fwService`. This is to avoid a namespace collision with the remote firmware library (`"chromiumos/tast/remote/firmware"`)—otherwise, both would be called `firmware`.

Next, declare the BIOS service as a ServiceDep in the test's initializiation:

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
	flags, err := bios.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get GBB flags: ", err)
	}
	s.Log("Clear GBB flags: ", flags.Clear)
	s.Log("Set GBB flags:   ", flags.Set)
}
```

At this point (after running `gofmt`), your test file should look like the following:

```go
// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	fwService "chromiumos/tast/services/firmware"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Codelab,
		Desc: "Demonstrates common functionality for remote firmware tests",
		Contacts: []string{
			"me@chromium.org",     // Test author
			"my-team@chromium.org, // Backup mailing list
		},
		Data:         []string{firmware.ConfigFile}
		Attr:         []string{"group:firmware", "firmware_experimental"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		ServiceDeps: []string{"tast.cros.firmware.BiosService"},
		Var:          []string{"servo"},
	})
}

func Codelab(ctx context.Context, s *testing.State) {
	r := reporters.New(s.DUT())
	board, err := r.Board(ctx)
	if err != nil {
		s.Fatal("Failed to report board: ", err)
	}
	model, err := r.Model(ctx)
	if err != nil {
		s.Fatal("Failed to report model: ", err)
	}
	s.Logf("Reported board=%s, model=%s", board, model)

	cfg, err := firmware.NewConfig(s.DataPath(firmware.ConfigFile), board, model)
	if err != nil {
		s.Fatal("Failed to create config file: ", err)
	}
	s.Log("This DUT's mode-switcher type is: ", cfg.ModeSwitcherType)

	// Set up Servo
	dut := s.DUT()
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	// Get the DUT's ec_board via Servo
	ecBoard, err := pxy.Servo().GetString(ctx, servo.ECBoard)
	if err != nil {
		s.Fatal("getting ec_board control from servo:", err)
	}
	s.Log("EC Board: ", ecBoard)

	// Connect to RPC
	cl, err := rpc.Dial(ctx, dut, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	bios := fwService.NewBiosServiceClient(cl.Conn)
	flags, err := bios.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to get GBB flags: ", err)
	}
	s.Log("Clear GBB flags: ", flags.Clear)
	s.Log("Set GBB flags:   ", flags.Set)
}
```

[built-in gRPC support]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/writing_tests.md#Remote-procedure-calls-with-gRPC
[BIOS service]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/services/cros/firmware/bios_service.proto

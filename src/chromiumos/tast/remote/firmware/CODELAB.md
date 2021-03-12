# Tast FAFT Codelab: Remote Firmware Tests

> This document assumes that you've gone already through [A Tour of Go], [Codelab #1] and [Codelab #2].

This codelab follows the creation of a remote firmware test in Tast. In doing so, we'll learn how to do the following:

* Schedule the test to run during a FAFT suite
* Skip the test on DUTs that don't have a Chrome EC
* Collect information about the DUT via `firmware.Reporter`
* Read fw-testing-configs values via `firmware.Config`
* Send Servo commands
* Send RPC commands to the DUT
* Manage common firmware structures via `firmware.Helper`
* Boot the DUT into recovery/developer mode via `firmware.ModeSwitcher`
* Verify the DUT state at the start and end of the test via `firmare.Pre`

The test we build will be a bit of a hodge-podge so that we can use all of the above features. Additionally, we will demonstrate a few different ways of initializing certain structures, so working through the codelab will teach you more than just studying the final code.

[A Tour of Go]: https://tour.golang.org/
[Codelab #1]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/codelab_1.md
[Codelab #2]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/codelab_2.md

## Boilerplate

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

Notice the `Attr` line in the above snippet. In previous codelabs, you probably used the attributes `"group:mainline"` and `"informational"`. Those attributes cause tests to run in the CQ. However, most firmware tests are very expensive to run, and might not be appropriate to run in the CQ. You can read more about effective CQ usage on-corp at [go/effective-cq]. Additionally, most firmware tests should be run on the `faft-test` device pool, unlike the mainline tests.

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

In Tast, we access that JSON as a [data file]. The path to that data-file is exported in the remote `firmware` library as [`firmware.ConfigFile`].

To use that data file in our test, we first have to import the remote `firmware` library, and declare that our test uses the data file `firmware.ConfigFile`:

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
[data files]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/writing_tests.md#Data-files
[`firmware.ConfigFile`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/firmware/config.go?q=ConfigFile
[`config.go`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/firmware/config.go
[`firmware.NewConfig`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/remote/firmware/config.go?q=%22func%20NewConfig%22
[`s.DataPath`]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/writing_tests.md#Data-files

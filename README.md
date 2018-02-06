# Tast (tests)

This repository contains integration tests that are run by [Tast](../tast/).

## Directory structure

*   [`src/chromiumos/tast/`](src/chromiumos/tast/)
      *   [`local/`](src/chromiumos/tast/local/) - Code related to local (i.e.
          on-device or "client") tests.
          *   [`bundles/`](src/chromiumos/tast/local/bundles/) - Local test
              bundles.
              *   [`cros/`](src/chromiumos/tast/local/bundles/cros/) - The
                  "cros" local test bundle, containing standard Chrome OS tests.
                  Tests are packaged by category.
          *   `...` - Packages used only by local tests.
      *   [`remote/`](src/chromiumos/tast/remote/) - Code related to remote
          (i.e. off-device or "server") tests.
          *   [`bundles/`](src/chromiumos/tast/remote/bundles/) - Remote test
              bundles.
              *   [`cros/`](src/chromiumos/tast/remote/bundles/cros/) - The
                  "cros" remote test bundle, containing standard Chrome OS
                  tests. Tests are packaged by category.
          *   `...` - Packages used only by remote tests.

Shared code, the main `tast` executable, the `local_test_runner` and
`remote_test_runner` executables responsible for running bundles, and
documentation are located in the [tast](../tast/) repository.

[![GoDoc](https://godoc.org/chromium.googlesource.com/chromiumos/platform/tast-tests.git/src?status.svg)](https://godoc.org/chromium.googlesource.com/chromiumos/platform/tast-tests.git/src)

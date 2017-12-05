# Tast (tests)

This repository contains integration tests that are run by [Tast](../tast/).

## Directory structure

*   [`src/chromiumos/`](src/chromiumos/)
    *   [`cmd/`](src/chromiumos/cmd/) - Test-related executables.
        *   [`local_tests/`](src/chromiumos/cmd/local_tests/) - `main` package for
            the `local_tests` executable containing "local" tests, i.e. ones that
            run on-device.
        *   [`remote_tests/`](src/chromiumos/cmd/remote_tests/) - `main` package for
            the `remote_tests` executable containing "remote" tests, i.e. ones that
            run off-device.
    *   [`tast/`](src/chromiumos/tast/)
        *   [`local/`](src/chromiumos/tast/local/) - Code related to local
            tests.
            *   [`tests/`](src/chromiumos/tast/local/tests) - Local tests,
                packaged by category.
            *   `...` - Packages used only by local tests.
        *   [`remote/`](src/chromiumos/tast/remote/) - Code related to remote
            tests.
            *   [`tests/`](src/chromiumos/tast/remote/tests/) - Remote tests,
                packaged by category.
            *   `...` - Packages used only by remote tests.

Shared code, the main `tast` executable, and documentation are located in the
[tast](../tast/) repository.

[![GoDoc](https://godoc.org/chromium.googlesource.com/chromiumos/platform/tast-tests.git/src?status.svg)](https://godoc.org/chromium.googlesource.com/chromiumos/platform/tast-tests.git/src)

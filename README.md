# Tast (tests)

This repository contains integration tests that are run by [Tast](../tast/).

## Directory structure

*   [`src/chromiumos/tast/`](src/chromiumos/tast/)
    *   [`local/`](src/chromiumos/tast/local/) - `main` package for the
        `local_tests` executable containing "local" tests, i.e. ones that run
        on-device.
        *   [`tests/`](src/chromiumos/tast/local/tests) - Local tests, packaged
            by category.
        *   `...` - Packages used only by local tests.
    *   [`remote/`](src/chromiumos/tast/remote/) - `main` package for the
        `remote_tests` executable containing "remote" tests, i.e. ones that run
        off-device.
        *   [`tests/`](src/chromiumos/tast/remote/tests/) - Remote tests,
            packaged by category.
        *   `...` - Packages used only by remote tests.

Shared code, the main `tast` executable, and documentation are located in the
[tast](../tast/) repository.

Package documentation is available at [godoc.org].

[godoc.org]: https://godoc.org/chromium.googlesource.com/chromiumos/platform/tast-tests.git/src/chromiumos/tast

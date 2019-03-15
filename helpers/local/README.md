This directory contains helper programs that are installed to test system images
so they can be used by local Tast tests.

Source files' names should take the form `category.TestName.prog.c`. They are
compiled to e.g. `category.TestName.prog` and installed to
`/usr/local/libexec/tast/helpers/local/cros` in test system images.

Please do not add new files to this directory if you can avoid it. Spreading a
test's logic across multiple locations makes it harder to understand and to
modify.

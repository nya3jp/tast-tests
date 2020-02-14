/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.print;

import android.app.Activity;
import android.content.Context;
import android.os.Bundle;
import android.print.PrintAttributes;
import android.print.PrintManager;

/**
 * Main activity for the ArcPrintTest app.
 *
 * <p>Used by tast tests to launch ARC printing. Provides a test PrintDocumentAdapter to generate a
 * new print document when print settings are changed.
 */
public class MainActivity extends Activity {
    public static final String LOG_TAG = MainActivity.class.getSimpleName();

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        setContentView(R.layout.main_activity);

        // Set the job name, which will be displayed in the print queue.
        String jobName = getString(R.string.app_name) + " Document";

        // Start a print job, passing in a PrintDocumentAdapter implementation to handle the
        // generation of a print document.
        PrintManager printManager = (PrintManager) getSystemService(Context.PRINT_SERVICE);
        printManager.print(jobName, new TestPrintDocumentAdapter(this), (PrintAttributes) null);
    }
}

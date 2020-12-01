/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.gameperformance;

import java.io.BufferedReader;
import java.io.FileWriter;
import java.io.IOException;
import java.io.InputStreamReader;

import android.app.Instrumentation;
import android.os.AsyncTask;
import android.os.ParcelFileDescriptor;
import android.util.Log;

/**
 * Helper that runs atrace command for required duration and category. Results are read from the
 * output of atrace and serialized to the provided file. We cannot use direct atrace to file because
 * atrace is executed in UI automator context and analysis is done in test context. In last case
 * output file is not accessible from the both contexts.
 */
public class ATraceRunner extends AsyncTask<Void, Integer, Boolean> {
    private static final String TAG = "ATraceRunner";

    // Report that atrace is done.
    public interface Delegate {
        public void onProcessed(boolean success);
    }

    private final Instrumentation mInstrumentation;
    private final String mOutput;
    private final int mTimeInSeconds;
    private final String mCategory;
    private final Delegate mDelegate;

    public ATraceRunner(
            Instrumentation instrumentation,
            String output,
            int timeInSeconds,
            String category,
            Delegate delegate) {
        mInstrumentation = instrumentation;
        mOutput = output;
        mTimeInSeconds = timeInSeconds;
        mCategory = category;
        mDelegate = delegate;
    }

    @Override
    protected Boolean doInBackground(Void... params) {
        BufferedReader bufferedReader = null;
        FileWriter writer = null;
        try {
            // Run the command.
            final String cmd = "atrace -t " + mTimeInSeconds + " " + mCategory;
            Log.i(TAG, "Running atrace... " + cmd);
            writer = new FileWriter(mOutput);
            final ParcelFileDescriptor fd =
                    mInstrumentation.getUiAutomation().executeShellCommand(cmd);
            bufferedReader =
                    new BufferedReader(
                            new InputStreamReader(
                                    new ParcelFileDescriptor.AutoCloseInputStream(fd)));
            String line;
            while ((line = bufferedReader.readLine()) != null) {
                writer.write(line);
                writer.write("\n");
            }
            Log.i(TAG, "Running atrace... DONE");
            return true;
        } catch (IOException e) {
            Log.i(TAG, "atrace failed", e);
            return false;
        } finally {
            Utils.closeQuietly(bufferedReader);
            Utils.closeQuietly(writer);
        }
    }

    @Override
    protected void onPostExecute(Boolean result) {
        mDelegate.onProcessed(result);
    }
}

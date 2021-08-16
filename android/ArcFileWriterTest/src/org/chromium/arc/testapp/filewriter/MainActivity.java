/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.filewriter;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.net.Uri;
import android.os.Bundle;
import android.util.Log;
import android.widget.TextView;

import java.io.FileNotFoundException;
import java.io.IOException;
import java.io.OutputStream;

/**
 * Main activity for the ArcFileWriterTest app.
 *
 * <p>Used by tast test to write a file in FilesApp. A constant string is written to the file
 * specified by a URI. Result (success/failure) is reported in the corresponding text field.
 */
public class MainActivity extends Activity {
    public static final String TAG = "ArcFileWriterTest";

    // Corresponds to chromiumos/tast/local/bundles/cros/arc/storage.ExpectedFileContent.
    private static final byte[] CONTENT_TO_WRITE = "this is a test".getBytes();

    private static final String RESULT_SUCCESS = "Success";
    private static final String RESULT_FAILURE = "Failure";

    private TextView mResult;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        setContentView(R.layout.main_activity);
        mResult = findViewById(R.id.result);

        Intent intent = getIntent();
        String action = intent.getAction();
        Uri uri = intent.getData();

        try (OutputStream output = getContentResolver().openOutputStream(uri)) {
            output.write(CONTENT_TO_WRITE);
            mResult.setText(RESULT_SUCCESS);
        } catch (FileNotFoundException e) {
            Log.e(TAG, "Cannot open uri " + uri.toString() + " for write: " + e);
            mResult.setText(RESULT_FAILURE);
        } catch (IOException e) {
            Log.e(TAG, "Error writing to uri " + uri.toString() + ": " + e);
            mResult.setText(RESULT_FAILURE);
        }
    }
}

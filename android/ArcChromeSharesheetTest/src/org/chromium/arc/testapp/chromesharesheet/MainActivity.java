/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.chromesharesheet;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.net.Uri;
import android.os.Bundle;
import android.util.Log;
import android.widget.TextView;

import java.io.FileNotFoundException;
import java.io.IOException;
import java.io.InputStream;
import java.util.Scanner;

/**
 * Main activity for the ArcChromeSharesheetTest app.
 *
 * Use by the arc.Sharesheet test to ensure ARC app receives intents.
 */
public class MainActivity extends Activity {
    public static final String LOG_TAG = MainActivity.class.getSimpleName();

    private TextView mAction;
    private TextView mUri;
    private TextView mfileContent;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        setContentView(R.layout.main_activity);
        mAction = findViewById(R.id.action);
        mUri = findViewById(R.id.uri);
        mfileContent = findViewById(R.id.file_content);

        processIntent();
    }

    private void processIntent() {
        Log.i(LOG_TAG, "Processing intent");
        Intent intent = getIntent();
        String action = intent.getAction();
        String type = intent.getType();
        Uri fileUri = (Uri) intent.getParcelableExtra(Intent.EXTRA_STREAM);

        mAction.setText(action);

        if (fileUri != null) {
            mUri.setText(fileUri.toString());
        }

        try (InputStream input = getContentResolver().openInputStream(fileUri);
                Scanner sc = new Scanner(input)) {
            StringBuffer sb = new StringBuffer();
            while(sc.hasNext()) {
                sb.append(sc.nextLine());
            }

            mfileContent.setText(sb.toString());
            Log.i(LOG_TAG, "File content = " + sb.toString());
        } catch (FileNotFoundException e) {
            Log.e(LOG_TAG, "Cannot open uri " + fileUri.toString(), e);
            finish();
        } catch (IOException e) {
            Log.e(LOG_TAG, "Error reading uri " + fileUri.toString(), e);
            finish();
        }
    }
}

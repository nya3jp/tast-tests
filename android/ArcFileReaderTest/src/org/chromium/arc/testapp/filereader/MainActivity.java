/*
 * Copyright 2020 The ChromiumOS Authors
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.filereader;

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
 * Main activity for the ArcFileReaderTest app.
 *
 * <p>Used by tast test to read file in FilesApp. File content is read and shown in TextView for
 * validation.
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
        mAction.setText(action);

        Uri uri = intent.getData();
        if (uri == null) {
          return;
        }
        mUri.setText(uri.toString());

        try (InputStream input = getContentResolver().openInputStream(uri);
            Scanner sc = new Scanner(input)) {
            StringBuffer sb = new StringBuffer();
            while(sc.hasNext()) {
                sb.append(sc.nextLine());
            }

            mfileContent.setText(sb.toString());
            Log.i(LOG_TAG, "File content = " + sb.toString());
        } catch (FileNotFoundException e) {
            Log.e(LOG_TAG, "Cannot open uri " + uri.toString() + ": " + e);
            finish();
        } catch (IOException e) {
            Log.e(LOG_TAG, "Error reading uri " + uri.toString() + ": " + e);
            finish();
        }
    }
}

/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
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
import android.content.BroadcastReceiver;
import android.content.IntentFilter;

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

    private BroadcastReceiver mMediaScanListener =
        new BroadcastReceiver() {
          @Override
          public void onReceive(Context context, Intent intent) {
            Log.e(LOG_TAG, "AIUEO: Received " + intent.getAction() + " for " + intent.getData());
          }
       };
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        setContentView(R.layout.main_activity);
        mAction = findViewById(R.id.action);
        mUri = findViewById(R.id.uri);
        mfileContent = findViewById(R.id.file_content);
        final IntentFilter filter = new IntentFilter();
        filter.addAction(Intent.ACTION_MEDIA_SCANNER_STARTED);
        filter.addAction(Intent.ACTION_MEDIA_SCANNER_FINISHED);
        filter.addDataScheme("file");
        // [EDITED] getContext().registerReceiver(mMediaScanListener, filter);
        registerReceiver(mMediaScanListener, filter);
        processIntent();
    }
    @Override
    public void onDestroy() {
        unregisterReceiver(mMediaScanListener);
        super.onDestroy();
    }
    private void processIntent() {
        Log.i(LOG_TAG, "Processing intent");
        Intent intent = getIntent();
        String action = intent.getAction();
        Uri uri = intent.getData();

        mAction.setText(action);
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

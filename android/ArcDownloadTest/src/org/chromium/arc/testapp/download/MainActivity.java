/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.download;

import android.Manifest;
import android.content.Context;
import android.app.DownloadManager;
import android.content.pm.PackageManager;
import android.os.Bundle;
import android.net.Uri;
import android.app.Activity;

import android.os.Environment;
import android.view.View;
import android.widget.Button;


public class MainActivity extends Activity {
    Button download;
    public static final int PERMISSION_WRITE = 0;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);
        download = findViewById(R.id.download);
        download.setOnClickListener(new View.OnClickListener() {

            @Override
            public void onClick(View v) {
                if (checkPermission()) {
                    String downloadPath = "https://www.gstatic.com/webp/gallery/1.jpg";
                    startDownload(downloadPath);
                }
            }

        });
    }

    //runtime storage permission
    public boolean checkPermission() {
        if(checkSelfPermission(
            Manifest.permission.READ_EXTERNAL_STORAGE) != PackageManager.PERMISSION_GRANTED
                || checkSelfPermission(Manifest.permission.WRITE_EXTERNAL_STORAGE) !=
                PackageManager.PERMISSION_GRANTED){

            // this will request for permission when user has not granted permission for the app
            requestPermissions(new String[]{
                Manifest.permission.READ_EXTERNAL_STORAGE,
                Manifest.permission.WRITE_EXTERNAL_STORAGE},
                PERMISSION_WRITE);
            return false;
        }
        return true;
    }

    public void onRequestPermissionsResult(
        int requestCode, String[] permissions, int[] grantResults) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults);
        if (requestCode==PERMISSION_WRITE &&
        grantResults.length > 0 && grantResults[0] == PackageManager.PERMISSION_GRANTED) {
            String downloadPath = "https://www.gstatic.com/webp/gallery/1.jpg";
            startDownload(downloadPath);
        }
    }

    private void startDownload(String downloadPath) {
        Uri uri = Uri.parse(downloadPath);
        DownloadManager.Request request = new DownloadManager.Request(uri);
        request.setAllowedNetworkTypes(
            DownloadManager.Request.NETWORK_MOBILE | DownloadManager.Request.NETWORK_WIFI);
        request.setNotificationVisibility(
            DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED);
        request.setTitle("Downloading a file");
        request.setDestinationInExternalPublicDir(
            Environment.DIRECTORY_DOWNLOADS, uri.getLastPathSegment());
        ((DownloadManager) getSystemService(Context.DOWNLOAD_SERVICE)).enqueue(request);
    }
}
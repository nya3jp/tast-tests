/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.connectivity;

import android.app.Activity;
import android.os.Bundle;
import android.view.View;
import android.widget.Button;
import android.widget.EditText;
import android.widget.TextView;
import java.net.HttpURLConnection;
import java.net.InetSocketAddress;
import java.net.Proxy;
import java.net.URL;
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;

/**
 * Activity for Chrome OS ARC++/ARCVM tast tests which shows the ARC++ global
 * proxy and tests the connectivity through the proxy from an ARC app by
 * connecting to a remote target.
 * Note: Starting the network request and getting the
 * result are two separate actions to allow intermediary steps like proxy
 * authentication.
 */
public class ConnectivityActivity extends Activity {
  ExecutorService mExecutor = Executors.newSingleThreadExecutor();
  Future<String> mResultFuture;
  /**
   * Performs a network request on a separate thread respecting the proxy set
   * through the http.proxyHost and http.proxyPort System properties.
   */
  private void doNetworkRequest(String urlValue) {
    mResultFuture = mExecutor.submit(new Callable<String>() {
      public String call() {
        try {
          URL url = new URL(urlValue);
          HttpURLConnection urlConnection =
              (HttpURLConnection)url.openConnection(new Proxy(
                  Proxy.Type.HTTP,
                  new InetSocketAddress(
                      System.getProperty("http.proxyHost"),
                      Integer.parseInt(System.getProperty("http.proxyPort")))));
          return urlConnection.getResponseCode() + "";
        } catch (Exception ex) {
          return ex.toString();
        }
      }
    });
  }

  @Override
  public void onCreate(Bundle savedInstanceState) {
    super.onCreate(savedInstanceState);
    setContentView(R.layout.main_activity);

    final TextView proxy_view = findViewById(R.id.global_proxy);
    proxy_view.setText(System.getProperty("http.proxyHost") + ":" +
                       System.getProperty("http.proxyPort"));

    final Button network_request_button =
        findViewById(R.id.network_request_button);
    network_request_button.setOnClickListener(new View.OnClickListener() {
      @Override
      public void onClick(View view) {
        final TextView url_view = findViewById(R.id.url);
        doNetworkRequest(url_view.getText().toString());
      }
    });

    // If the request was successful, updates the `result` text view with the
    // HTTP response code of the network request started via the
    // `network_request_button`. If the request failed, it will update `result`
    // with an error code or error message.
    final Button await_result_button = findViewById(R.id.await_result_button);
    await_result_button.setOnClickListener(new View.OnClickListener() {
      @Override
      public void onClick(View view) {
        String result;
        try {
          if (mResultFuture == null)
            result = "No network request in progress.";
          else
            result = mResultFuture.get(30, TimeUnit.SECONDS);
        } catch (Exception ex) {
          result = ex.toString();
        }
        final TextView resultView = findViewById(R.id.result);
        resultView.setText(result);
      }
    });
  }
}

/**
 * @fileoverview Contains the logic for interacting with the
 * NetworkDiagnosticsRoutines Mojo API.
 */
function() {
  return {
    publicVariable : null,

    privateVariable_ : null,

    publicMethod() {},

    privateMethod_() {},

    onPublicCallback() {},

    onPrivateCallback_() {},

    networkDiagnosticsRoutines_ : null,

    createNetworkDiagnosticsRoutinesRemote() {
      this.networkDiagnosticsRoutines_ = chromeos.networkDiagnostics.mojom.NetworkDiagnosticsRoutines.getRemote();
      /* Check for existence of onConnectionError callback. (Is this part of the
       * JS Mojo bindings)*/
      //this.networkDiagnosticsRoutines_.onConnectionError.addListener(() => this.networkDiagnosticsRoutines_ = null);
    },

    networkDiagnosticsRoutines() {
      return this.networkDiagnosticsRoutines_;
    },

    evaluateRoutine_(response) {
      return true;
    },

    lanConnectivity() {
      if (!this.networkDiagnosticsRoutines()) {
        this.createNetworkDiagnosticsRoutinesRemote();
      }
      //var func = () => this.networkDiagnosticsRoutines_.lanConnectivity();
      var verdict = this.networkDiagnosticsRoutines_.lanConnectivity();
      var promise = new Promise(function(resolve, reject) {
        // do a thing, possibly async, then…
        if (true) {
          resolve("Stuff worked!");
        } else {
          reject(Error("It broke"));
        }
      });
      promise.then(function(result) {
        console.log(result); // "Stuff worked!"
      }, function(err) {
        console.log(err); // Error: "It broke"
      });
      //func.then(response => this.evaluateRoutine_(response));
    },
  }
}

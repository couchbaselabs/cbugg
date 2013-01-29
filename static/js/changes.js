function ChangesCtrl($scope, $routeParams, $http, $rootScope, cbuggAuth) {
    
    $scope.changes = [];
    
    $scope.appendLog = function(change) {
        $scope.$apply(function(scope) {
            scope.changes.unshift(change);
          });
    }
    
    $scope.displayErrorMessage = function(message) {
        $scope.$apply(function(scope) {
            scope.errorMessage = message;
          });
    }
    
    // this reapplies the page every 30 seconds
    // so that the relative times don't get stale
    var interval = setInterval(function(){
        $scope.$apply();
     }, 30000);
    
    var loc = window.location, new_uri;
    if (loc.protocol === "https:") {
        new_uri = "wss:";
    } else {
        new_uri = "ws:";
    }
    new_uri += "//" + loc.host;
    new_uri += "/api/changes/";
    
    if (window["WebSocket"]) {
        conn = new WebSocket(new_uri);
        conn.onclose = function(evt) {
            $scope.displayErrorMessage("Your WebSocket was disconnected");
        }
        conn.onmessage = function(evt) {
            change = JSON.parse(evt.data)
            $scope.appendLog(change)
        }
    } else {
        $scope.displayErrorMessage("Your browser does not support WebSockets");
    }
    
}
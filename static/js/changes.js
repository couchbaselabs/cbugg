function ChangesCtrl($scope, $routeParams, $http, $rootScope, cbuggAuth) {

    $scope.changes = [];

    $scope.appendLog = function(change) {
        $scope.$apply(function(scope) {
            scope.changes.unshift(change);
          });
    };

    $scope.displayErrorMessage = function(message) {
        $scope.$apply(function(scope) {
            scope.errorMessage = message;
          });
    };

    // this reapplies the page every 30 seconds
    // so that the relative times don't get stale
    var interval = setInterval(function(){
        $scope.$apply();
     }, 30000);

    var loc = window.location;
    var new_uri = loc.protocol + "//" + loc.host + "/api/changes";

    var sock = new SockJS(new_uri);
    sock.onmessage = function(e) {
        $scope.appendLog(e.data);
    };
    sock.onclose = function() {
        $scope.displayErrorMessage("Your realtime stream was disconnected");
    };

}
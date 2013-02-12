function RealtimeStatusCtrl($scope, cbuggRealtime) {

    $scope.$on('ChangesOpen', function() {
        $scope.connected = "connected";
        $scope.message = "Connected to realtime changes";
    });

    $scope.$on('ChangesClosed', function(event, retry) {
        $scope.connected = "disconnected";
        $scope.message = "Disconnected from realtime changes.  Retrying in " + retry + " seconds.";
    });

}
var cbuggGrowl = angular.module('cbuggGrowl', []);
cbuggGrowl.factory('cbuggGrowl', function($rootScope) {

    var alerts = [];

    function createAlert(alert) {
        alerts.push(alert);
    }

    function closeGrowlAlert(alert) {
        var index = alerts.indexOf(alert);
        alerts.splice(index, 1);
    }

    $rootScope.$on('$routeChangeSuccess', function(evt, curr, prev) {
        var i = alerts.length;
        while(i--) {
            var alert = alerts[i];
            if(alert.context && alert.context !==  curr) {
                closeGrowlAlert(alert);
            }
        }
    });

    return {
        currentAlerts: alerts,
        createAlert: createAlert,
        closeGrowlAlert: closeGrowlAlert
    };

});

function GrowlAlertCtrl($scope, $location, cbuggGrowl) {

    $scope.alerts = cbuggGrowl.currentAlerts;

    $scope.closeGrowlAlert = function(alert) {
        cbuggGrowl.closeGrowlAlert(alert);
    };

}
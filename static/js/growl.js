var cbuggGrowl = angular.module('cbuggGrowl', []);
cbuggGrowl.factory('cbuggGrowl', function($rootScope) {

    var alerts = [];
    var alertsById = {};

    function createAlert(alert) {
        if(alert.id) {
            if(alertsById[alert.id]) {
                closeGrowlAlert(alertsById[alert.id]);
            }
            alertsById[alert.id] = alert;
        }
        alerts.push(alert);
    }

    function closeGrowlAlert(alert) {
        if(alert.id) {
            delete alertsById[alert.id];
        }
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

function GrowlAlertCtrl($scope, $route, cbuggGrowl) {

    $scope.alerts = cbuggGrowl.currentAlerts;

    $scope.closeGrowlAlert = function(alert) {
        cbuggGrowl.closeGrowlAlert(alert);
    };

    $scope.reload = function() {
        $route.reload();
    };

}
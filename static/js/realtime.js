var cbuggRealtime = angular.module('cbuggRealtime', []);

cbuggRealtime.factory('cbuggRealtime', function($rootScope, $http, $location, $timeout) {

    var recentChanges = [];

    // if the loggedin status changes, we want to restart the socket
    $rootScope.$watch('loggedin', function() {
        sock.close();
        sock = new SockJS(changesURI);
        sock.onopen = onOpen;
        sock.onmessage = onMessage;
        sock.onclose = onClose;
    });

    var changesURI = $location.protocol() + "://" + $location.host() + ":"
            + $location.port() + "/api/changes";
    var nextRetry = 30;

    var onOpen = function() {

        // clear out any old changes buffered
        // do not create new array
        // (we may have already returned reference to it)
        recentChanges.length = 0;

        // onOpen we need to send our auth cookie
        var authMessage = {
            "type" : "auth",
            "cookie" : $.cookie('cbugger')
        };

        sock.send(JSON.stringify(authMessage));
        nextRetry = 30;

        $rootScope.$apply(function() {
            $rootScope.$broadcast('ChangesOpen')
        });
    };

    var onClose = function() {
        $rootScope.$apply(function() {
            nextRetry = 2 * nextRetry;
            $rootScope.$broadcast('ChangesClosed', nextRetry);

            // this auto-reconnect with backoff is nice, but a bit problematic right now
            // ie, when logged-in status changes, there can be outstanding timeouts, etc
            // and you can end up with more than 1 socket connect, and you get the same
            // events multiple times

            //            $timeout(function() {
            //                sock = new SockJS(changesURI);
            //                sock.onopen = onOpen;
            //                sock.onmessage = onMessage;
            //                sock.onclose = onClose;
            //            }, nextRetry * 1000);
        });
    };

    var onMessage = function(e) {
        $rootScope.$apply(function() {
            // keep a buffer of the last 10 changes we've seen
            recentChanges.unshift(e.data);
            if(recentChanges.length > 10) {
                recentChanges.length = 10;
            }

            $rootScope.$broadcast('Change', e.data);
        });
    };

    var sock = new SockJS(changesURI);
    sock.onopen = onOpen;
    sock.onmessage = onMessage;
    sock.onclose = onClose;

    return {
        recentChanges: function() { return recentChanges; }
    };
});
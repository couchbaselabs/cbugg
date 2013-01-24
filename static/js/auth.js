var cbuggAuth = angular.module('cbuggAuth', []);
cbuggAuth.factory('cbuggAuth', function($rootScope, $http, bAlert) {
    var auth = {
        loggedin: false,
        username: "",
        gravatar: "",
        authtoken: ""
    };

    if(navigator.id) {
        navigator.id.watch({
            onlogin: function(assertion) {
                $http.post('/auth/login', "assertion="+assertion+"&audience=" +
                    encodeURIComponent(location.protocol+"//"+location.host),
                    {headers: {"Content-Type": "application/x-www-form-urlencoded"}}).
                success(function(res) {
                    auth.loggedin = true;
                    auth.username = res.email;
                    auth.gravatar = res.emailmd5;
                    auth.authtoken = "";
                    $rootScope.loggedin = true;
                }).
                error(function(res, err) {
                    bAlert("Error", "Couldn't log you in.", "error");
                });
            },
            onlogout: function() {
                $rootScope.loggedin = false;
                auth.loggedin = false;
                auth.authtoken = "";
            }});
    }
    function fetchAuthToken() {
        $http.get("/api/me/token/").
            success(function(res) {
                auth.authtoken = res.token;
            });
    }
    function logout() {
        navigator.id.logout();
        $http.post('/auth/logout').
            success(function(res) {
                $rootScope.loggedin = false;
                $scope.loggedin = false;
            }).
        error(function(res) {
            bAlert("Error", "Problem logging out.", "error");
            $rootScope.loggedin = false;
            $scope.loggedin = false;
        });
    }
    function login() {
        navigator.id.request();
    }
    function getAuth() {
        return auth;
    }
    return {
        login: login,
        logout: logout,
        get: getAuth
    };
});

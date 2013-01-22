angular.module('cbuggFilters', []).
    filter('markdownify', function() {
        return function(string) {
            if(!string) { return ""; }
            return marked(string);
        };
    }).
    filter('relDate', function() {
        return function(dstr) {
            return moment(dstr).fromNow();
        };
    }).
    filter('calDate', function() {
        return function(dstr) {
            return moment(dstr).calendar();
        };
    }).
    filter('bytes', function() {
        return function(s) {
            if (s < 10) {
                return s + "B";
	        }
	        var e = parseInt(Math.floor(Math.log(s) / Math.log(1024)));
            var sizes = ["B", "KB", "MB", "GB", "TB", "PB", "EB"];
	        var suffix = sizes[parseInt(e)];
	        var val = s / Math.pow(1024, Math.floor(e));
            return val.toFixed(2) + suffix;
        };
    });

angular.module('cbugg', ['cbuggFilters', 'cbuggEditor', 'cbuggAlert', 'ui', '$strap.directives']).
    config(['$routeProvider', function($routeProvider) {
        $routeProvider.
            when('/bug/:bugId', {templateUrl: 'partials/bug.html',
                                 controller: 'BugCtrl'}).
            when('/state/:stateId', {templateUrl: 'partials/buglist.html',
                                     controller: 'BugsByStateCtrl'}).
            when('/user/:userId/:stateId', {templateUrl: 'partials/buglist.html',
                                     controller: 'BugsByUserStateCtrl'}).
            when('/search/:query', {templateUrl: 'partials/searchresults.html',
                                         controller: 'SearchResultsCtrl'}).
            when('/statecounts', {templateUrl: 'partials/statecounts.html',
                                  controller: 'StatesByCountCtrl'}).
            otherwise({redirectTo: '/statecounts'});
    }]);

function StatesByCountCtrl($scope, $http, $rootScope) {
    $rootScope.title = "Home"
    $http.get('/api/state-counts').success(function(stateCounts) {
        $http.get("/api/states/").success(function(allstates) {
            var scopeMap = _.object(_.pluck(allstates, 'name'), allstates);
            $scope.states = _.sortBy(_.pairs(stateCounts.states),
                                     function(n) {
                                         return scopeMap[n[0]].order;
                                     });
        });
            });
    $http.get('/api/recent/').success(function(data) {
        $scope.recent = _.first(data, 10);
    });
}

function bugListDataPrep(data) {
    var grouped = _.pairs(_.groupBy(data, function(e) { return e.Value.Owner.md5; }));
    var nameMap = _.object(_.map(data, function(e){return e.Value.Owner.md5;}),
                           _.map(data, function(e){return e.Value.Owner.email;}));
    return _.sortBy(grouped, function(n) { return nameMap[n[0]]; });
}

function BugsByStateCtrl($scope, $routeParams, $http, $rootScope) {
    $rootScope.title = "State " + $routeParams.stateId;
    $scope.liststate = $routeParams.stateId;
    $http.get('/api/bug/?state=' + $routeParams.stateId).success(function(data) {
        $scope.grouped_bugs = bugListDataPrep(data);
    });
}

function BugsByUserStateCtrl($scope, $routeParams, $http, $rootScope) {
    $rootScope.title = "User " + $routeParams.userId + " State " + $routeParams.stateId;
    $scope.listuser = $routeParams.userId;
    $scope.liststate = $routeParams.stateId;
    $http.get('/api/bug/?user=' + $routeParams.userId +
              '&state=' + $routeParams.stateId).success(function(data) {
                  $scope.grouped_bugs = bugListDataPrep(data);
    });
}

function FakeLoginCtrl($scope) {
    $rootScope.loginscope = $scope;
    $scope.login = function() {
        $rootScope.loggedin = true;
        $scope.loggedin = true;
        $scope.username = "Test User";
        $scope.gravatar = "eee3b47a26586bb79e0a832466c066be";
    };
    $scope.logout = function() {
        $rootScope.loggedin = false;
        $scope.loggedin = false;
    };
}

function LoginCtrl($scope, $http, $rootScope, bAlert) {
    $rootScope.loginscope = $scope;
    $scope.authtoken = null;

    navigator.id.watch({
        onlogin: function(assertion) {
            $http.post('/auth/login', "assertion="+assertion+"&audience=" +
                encodeURIComponent(location.protocol+"//"+location.host),
                {headers: {"Content-Type": "application/x-www-form-urlencoded"}}).
                success(function(res) {
                    $scope.loggedin = true;
                    $scope.username = res.email;
                    $scope.gravatar = res.emailmd5;
                    $scope.authtoken = "";
                    $rootScope.loggedin = true;
                }).
                error(function(res, err) {
                    bAlert("Error", "Couldn't log you in.", "error");
                });
        },
        onlogout: function() {
            $rootScope.loggedin = false;
            $scope.loggedin = false;
            $scope.authtoken = "";
        }
    });

    $scope.getAuthToken = function() {
        $http.get("/api/me/token/").
            success(function(res) {
                $scope.authtoken = res.token;
            });
    };

    $scope.logout = function() {
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
    };

    $scope.login = function() {
        navigator.id.request();
    };
}

function SearchCtrl($scope, $http, $rootScope, $location) {
    $rootScope.loginscope = $scope;

    $scope.search = function() {
        $location.path("/search/" + $scope.query);
    };
}

function SearchResultsCtrl($scope, $routeParams, $http, $rootScope) {

    $scope.searchInProgress = true;
    $scope.searchError = false;
    $scope.page = 1;
    $scope.rpp = 10;
    $rootScope.title = "Search"

    $scope.jumpToPage = function(pageNum, $event) {
        if($event != null) {
            $event.preventDefault();
        }
        $scope.page = pageNum;
        $http.post('/api/search/?query=' + $routeParams.query + '&from=' + (($scope.page - 1) * $scope.rpp)).success(function(data) {
            $scope.shards = data._shards;
            $scope.results = data.hits.hits;
            $scope.facets = data.facets;
            $scope.total = data.hits.total;
            $scope.computeValidPages();
            $scope.verifyAllSearchShards();
            $scope.searchInProgress = false;
        }).error(function(data, status, headers, config){
            $scope.searchWarning = data;
            $scope.searchError = true;
            $scope.searchInProgress = false;
        });
    };

    $scope.verifyAllSearchShards = function() {
        if($scope.shards.total != $scope.shards.successful) {
            $scope.searchWarning = "Search only contains results from " + $scope.shards.successful + " of " + $scope.shards.total + " shards";
        }
    }

    $scope.computeValidPages = function() {
        $scope.numPages = Math.ceil($scope.total / $scope.rpp);
        $scope.validPages = new Array();
        for(i=1;i<=$scope.numPages;i++) {
            $scope.validPages.push(i);
        }
        $scope.firstResult = (($scope.page - 1) * $scope.rpp) + 1;
        if($scope.firstResult > $scope.total) {
            $scope.firstResult = $scope.total;
        }
        $scope.lastResult = $scope.firstResult + $scope.rpp - 1;
        if($scope.lastResult > $scope.total) {
            $scope.lastResult = $scope.total;
        }
    };

    $scope.query = $routeParams.query;
    $scope.jumpToPage(1, null);

}

function SimilarBugCtrl($scope, $http, $rootScope, $location) {

    $scope.debouncedLookupSimilar = _.debounce(function(){$scope.lookupSimilar()}, 500);

    $scope.lookupSimilar = function() {
        if($scope.bugTitle != "") {
            $http.post('/api/search/?query=' + $scope.bugTitle).success(function(data) {
                $scope.similarBugs = data.hits.hits;
            }).error(function(data, status, headers, config){
                // in this case we remove anything that might be in the variable
                $scope.similarBugs = [];
            });
        } else {
            $scope.similarBugs = [];
        }

    }
}

function PingCtrl($scope, $rootScope, $http, bAlert) {
    var loginscope = $rootScope.loginscope;
    $(".pinguserinput").focus();
    //Should actually factor getUsers out into a service instead of do this
    //hacky parent scope thing..
    $(".pinguserinput").typeahead({source: $scope.$parent.getUsers});
    $scope.pingUser = function() {
        var user = $(".pinguserinput").val();
        var bug = $scope.$parent.bug;
        if(user) {
            $http.post("/api/bug/" + bug.id + "/ping/", "to=" + encodeURIComponent(user),
                       {headers: {"Content-Type": "application/x-www-form-urlencoded"}})
                .error(function(data, code) {
                    bAlert("Error " + code, "Couldn't ping " + user + ": " + data, "error");
                })
                .success(function(data) {
                    $scope.$parent.comments.push({
                        type: 'ping',
                        from: {md5: loginscope.gravatar,
                               email: loginscope.username.match(/[^@]+/)[0]},
                        to: data
                    });
                });
        }
        $scope.dismiss();
    }
}

function RemindCtrl($scope, $rootScope, $http, bAlert) {
    var loginscope = $rootScope.loginscope;
    $(".remindmeinput").focus();
    $scope.remindMe = function() {
        var user = $(".remindmeinput").val();
        var bug = $scope.$parent.bug;
        if(user) {
            $http.post("/api/bug/" + bug.id + "/remindme/", "when=" + encodeURIComponent(user),
                       {headers: {"Content-Type": "application/x-www-form-urlencoded"}})
                .error(function(data, code) {
                    bAlert("Error " + code, "Failed to schedule your reminder.");
                });
        }
        $scope.dismiss();
    };
}

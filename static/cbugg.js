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

angular.module('cbugg', ['cbuggFilters', 'cbuggAuth', 'cbuggEditor', 'cbuggAlert',
                         'ui', '$strap.directives']).
    config(['$routeProvider', '$locationProvider',
            function($routeProvider, $locationProvider) {
        $routeProvider.
            when('/bug/:bugId', {templateUrl: '/static/partials/bug.html',
                                 controller: 'BugCtrl'}).
            when('/state/:stateId', {templateUrl: '/static/partials/buglist.html',
                                     controller: 'BugsByStateCtrl'}).
            when('/user/:userId/:stateId', {templateUrl: '/static/partials/buglist.html',
                                     controller: 'BugsByUserStateCtrl'}).
            when('/search/:query', {templateUrl: '/static/partials/searchresults.html',
                                         controller: 'SearchResultsCtrl'}).
            when('/statecounts', {templateUrl: '/static/partials/statecounts.html',
                                  controller: 'StatesByCountCtrl'}).
            when('/tag/:tagname', {templateUrl: '/static/partials/tag.html',
                                  controller: 'TagCtrl'}).
            otherwise({redirectTo: '/statecounts'});
        $locationProvider.html5Mode(true);
        $locationProvider.hashPrefix('!');
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

function LoginCtrl($scope, $http, $rootScope, bAlert, cbuggAuth) {
    $rootScope.$watch('loggedin', function() { $scope.auth = cbuggAuth.get(); });

    $scope.getAuthToken = function() {
        $http.get("/api/me/token/").
            success(function(res) {
                $scope.authtoken = res.token;
            });
    };

    $scope.logout = cbuggAuth.logout;
    $scope.login = cbuggAuth.login;
}

function SearchCtrl($scope, $http, $rootScope, $location, cbuggAuth) {
    $rootScope.$watch('loggedin', function() { $scope.auth = cbuggAuth.get(); });

    $scope.search = function() {
        $location.path("/search/" + $scope.query);
    };
}

function SearchResultsCtrl($scope, $routeParams, $http, $rootScope) {

    $scope.searchInProgress = true;
    $scope.searchError = false;
    $scope.page = 1;
    $scope.rpp = 10;
    $scope.pageSizes = [ 10, 30, 50, 100 ];
    $scope.filterStatus = [];
    $scope.filterTags = [];
    $rootScope.title = "Search";

    $scope.jumpToPage = function(pageNum, $event) {
        if ($event != null) {
            $event.preventDefault();
        }
        $scope.page = pageNum;
        $http.post(
            '/api/search/?query=' + $routeParams.query + '&from='
                    + (($scope.page - 1) * $scope.rpp) + '&size=' + $scope.rpp
                    + "&status=" + $scope.filterStatus.join(",") + "&tags="
                    + $scope.filterTags.join(",")).success(function(data) {
            $scope.shards = data._shards;
            $scope.results = data.hits.hits;
            $scope.facets = data.facets;
            $scope.total = data.hits.total;
            $scope.computeValidPages();
            $scope.verifyAllSearchShards();
            $scope.searchInProgress = false;
        }).error(function(data, status, headers, config) {
            $scope.searchWarning = data;
            $scope.searchError = true;
            $scope.searchInProgress = false;
        });
    };

    $scope.verifyAllSearchShards = function() {
        if ($scope.shards.total != $scope.shards.successful) {
            $scope.searchWarning = "Search only contains results from "
                    + $scope.shards.successful + " of " + $scope.shards.total
                    + " shards";
        }
    }

    $scope.computeValidPages = function() {
        $scope.numPages = Math.ceil($scope.total / $scope.rpp);
        $scope.validPages = new Array();
        for (i = 1; i <= $scope.numPages; i++) {
            $scope.validPages.push(i);
        }
        $scope.firstResult = (($scope.page - 1) * $scope.rpp) + 1;
        if ($scope.firstResult > $scope.total) {
            $scope.firstResult = $scope.total;
        }
        $scope.lastResult = $scope.firstResult + $scope.rpp - 1;
        if ($scope.lastResult > $scope.total) {
            $scope.lastResult = $scope.total;
        }
    };

    $scope.updateStatusFilter = function(val) {
        pos = $scope.filterStatus.indexOf(val);
        if (pos == -1) {
            $scope.filterStatus.push(val)
        } else {
            $scope.filterStatus.splice(pos, 1);
        }
        $scope.jumpToPage(1, null);
    }

    $scope.updateTagFilter = function(val) {
        pos = $scope.filterTags.indexOf(val);
        if (pos == -1) {
            $scope.filterTags.push(val)
        } else {
            $scope.filterTags.splice(pos, 1);
        }
        $scope.jumpToPage(1, null);
    }

    $scope.changeRpp = function(size, $event) {
        $scope.rpp = size;
        $scope.jumpToPage(1, $event);
    }

    $scope.query = $routeParams.query;
    $scope.jumpToPage(1, null);

}

function SimilarBugCtrl($scope, $http, $rootScope, $location) {

    $scope.similarBugs = [];
    $scope.debouncedLookupSimilar = _.debounce(function(){$scope.lookupSimilar()}, 500);

    $scope.lookupSimilar = function() {
        if($scope.bugTitle) {
            $http.post('/api/bugslike/?query=' + $scope.bugTitle).success(function(data) {
                $scope.similarBugs = data.hits.hits;
            }).error(function(data, status, headers, config){
                // in this case we remove anything that might be in the variable
                $scope.similarBugs = [];
            });
        } else {
            $scope.$apply(function(scope) {
                scope.similarBugs = [];
              });
        }

    }
}

function PingCtrl($scope, $rootScope, $http, bAlert, cbuggAuth) {
    $rootScope.$watch('loggedin', function() { $scope.auth = cbuggAuth.get(); });
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
                        from: {md5: $scope.auth.gravatar,
                               email: $scope.auth.username.match(/[^@]+/)[0]},
                        to: data
                    });
                });
        }
        $scope.dismiss();
    }
}

function RemindCtrl($scope, $rootScope, $http, bAlert, cbuggAuth) {
    $rootScope.$watch('loggedin', function() { $scope.auth = cbuggAuth.get(); });
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

function TagCtrl($scope, $routeParams, $http, $rootScope, $timeout, $location, bAlert, cbuggAuth) {
    $rootScope.title = "Tag " + $routeParams.tagname;
    $scope.tag = null;
    $scope.states = [];
    $scope.subscribed = false;
    $scope.subcount = 0;

    var checkSubscribed = function() {
        if($scope.tag) {
            $scope.subcount = $scope.tag.subscribers.length;
            _.forEach($scope.tag.subscribers, function(el) {
                $scope.subscribed |= ($scope.auth.gravatar == el.md5);
            });
        }
    };

    $rootScope.$watch("loggedin", function(nv, ov) {
        $scope.auth = cbuggAuth.get();
        checkSubscribed();
    });

    $scope.subscribe = function() {
        $http.post('/api/tags/' + $scope.tag.name + '/sub/');
        $scope.subscribed = true;
        $scope.subcount++;
        return false;
    };

    $scope.unsubscribe = function() {
        $http.delete('/api/tags/' + $scope.tag.name + '/sub/');
        $scope.subscribed = false;
        $scope.subcount--;
        return false;
    };

    $http.get('/api/tags/' + $routeParams.tagname + "/").success(function(taginfo) {
        $http.get("/api/states/").success(function(allstates) {
            $scope.tag = taginfo;
            var scopeMap = _.object(_.pluck(allstates, 'name'), allstates);
            $scope.states = _.sortBy(_.pairs(taginfo.states),
                                     function(n) {
                                         return scopeMap[n[0]].order;
                                     });

            $scope.subcount = taginfo.subscribers.length;
        });
    });
}

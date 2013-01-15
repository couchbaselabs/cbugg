angular.module('cbuggDirectives', [])
    .directive('cbMarkdown', function () {
        var converter = new Showdown.converter();
        var previewEditIcon = '<button class="btn btn-mini pull-right" ng-click="switchToEdit()">'+
                              'Edit <i class="icon-pencil"></i></button>';
        var previewTemplate = '<div ng-hide="isEditMode" class="well">'+previewEditIcon+'</div>';
        return {
            restrict:'E',
            scope:{},
            require:'ngModel',
            compile:function (tElement, tAttrs, transclude) {
                var initial = tElement.text();
                var saveText = tAttrs["savetext"];
                var modeflag = tAttrs["modeflag"];
                if(!saveText) { saveText = "Save"; }
                tElement.html('<div ng-class="{edithide: !isEditMode}">'+
                              '<textarea ui-codemirror="{theme:\'monokai\', '+
                              'mode: {name:\'markdown\'}}" ng-model="markdown">'+
                              '</textarea>Format with <a href="http://daringfireball.net/projects/markdown/syntax">Markdown</a>'+
                              '<button class="btn pull-right" ng-click="switchToPreview()">'+
                              saveText+'</button></div>');
                var editing = tAttrs["edit"];
                var previewOuterElement = angular.element(previewTemplate);
                var previewInnerElement = angular.element('<div></div>');
                tElement.append(previewOuterElement);
                previewOuterElement.append(previewInnerElement);

                return function (scope, element, attrs, model) {
                    scope.renderPreview = function() {
                        var makeHtml = converter.makeHtml(scope.markdown);
                        previewInnerElement.html(makeHtml);
                    }
                    scope.switchToPreview = function () {
                        model.$setViewValue(scope.markdown);
                        scope.renderPreview();
                        scope.isEditMode = false;
                        if(modeflag) {
                            scope.$parent[modeflag] = false;
                        }
                    }
                    scope.switchToEdit = function () {
                        scope.isEditMode = true;
                        if(modeflag) {
                            scope.$parent[modeflag] = true;
                        }
                    }
                    scope.$watch(attrs["ngModel"], function() {
                        var up = model.$modelValue;
                        if(up) {
                            scope.markdown = up;
                            scope.renderPreview();
                        }
                    })
                    scope.markdown=initial;
                    if(editing) {
                        scope.switchToEdit()
                    } else {
                        scope.switchToPreview()
                    }
                }
            }
        }
    });

function bAlert(heading, message, kind) {
    var kindclass = "";
    if(kind) {
        kindclass = "alert-" + kind;
    }
    $(".app").prepend(
            "<div class='alert fade in " + kindclass + "'>"+
            "<button type='button' class='close' data-dismiss='alert'>&times;</button>"+
            "<strong>" + heading + ":</strong> " + message + "</div>");
    $(".alert").alert();
}

angular.module('cbuggFilters', []).
    filter('markdownify', function() {
        var converter = new Showdown.converter();
        return function(string) {
            return converter.makeHtml(string)
        }
    }).
    filter('relDate', function() {
        return function(dstr) {
            return moment(dstr).fromNow();
        }
    }).
    filter('calDate', function() {
        return function(dstr) {
            return moment(dstr).calendar();
        }
    });

angular.module('cbugg', ['cbuggFilters', 'cbuggDirectives', 'ui']).
    config(['$routeProvider', function($routeProvider) {
        $routeProvider.
            when('/bug/:bugId', {templateUrl: 'partials/bug.html',
                                 controller: 'BugCtrl'}).
            when('/state/:stateId', {templateUrl: 'partials/buglist.html',
                                     controller: 'BugsByStateCtrl'}).
            when('/user/:userId/:stateId', {templateUrl: 'partials/buglist.html',
                                     controller: 'BugsByUserStateCtrl'}).
            when('/statecounts', {templateUrl: 'partials/statecounts.html',
                                  controller: 'StatesByCountCtrl'}).
            otherwise({redirectTo: '/statecounts'})

    }]);

function StatesByCountCtrl($scope, $http) {
    $http.get('/api/state-counts').success(function(data) {
        $scope.states = data.states;
    });
}

function BugsByStateCtrl($scope, $routeParams, $http) {
    $scope.liststate = $routeParams.stateId;
    $http.get('/api/bug/?state=' + $routeParams.stateId).success(function(data) {
        $scope.bugs = data;
    });
}

function BugsByUserStateCtrl($scope, $routeParams, $http) {
    $scope.listuser = $routeParams.userId;
    $scope.liststate = $routeParams.stateId;
    $http.get('/api/bug/?user=' + $routeParams.userId +
              '&state=' + $routeParams.stateId).success(function(data) {
        $scope.bugs = data;
    });
}

function BugCtrl($scope, $routeParams, $http, $rootScope) {
    var updateBug = function(field, newValue) {
        var bug = $scope.bug;
        if (!newValue) {
            newValue = bug[field];
        }
        if(bug && newValue) {
            $http.post('/api/bug/' + bug.id, "id=" + field + "&value=" + encodeURIComponent(newValue),
                       {headers: {"Content-Type": "application/x-www-form-urlencoded"}}).
            success(function(data) {
                $scope.bug = data;
            }).
            error(function(data, code) {
                bAlert("Error " + code, "could not update bug: " + data, "error")
            });
        }
    }

    $scope.allStates = ["new", "open", "resolved", "closed"];
    $scope.comments = [];
    $scope.draftcomment = "";


    $http.get('/api/bug/' + $routeParams.bugId).success(function(data) {
        $scope.bug = data.bug;
        $scope.history = data.history;
        $scope.history.reverse();
        $scope.$watch('bug.description', function(next, prev) {
            if($scope.bug && next !== prev) {
                updateBug("description");
            }
        });
        $scope.$watch('bug.status', function(next, prev) {
            if($scope.bug && next !== prev) {
                updateBug("status");
            }
        });
    });

    var checkComments = function (comments) {
        return _.map(comments, function(comment) {
            if($rootScope.loggedin &&
               $rootScope.loginscope.gravatar == comment.user.md5) {
                comment.mine = true;
            } else {
                comment.mine = false;
            }
            return comment;
        });
    }

    $rootScope.$watch("loggedin", function(nv, ov) {
        //Update delete button available on loggedinnness change
        $scope.comments = checkComments($scope.comments);
    });

    $http.get('/api/bug/' + $routeParams.bugId + '/comments/').success(function(data) {
        $scope.comments = checkComments(data);
    });

    $scope.killTag = function(kill) {
        $scope.bug.tags = _.filter($scope.bug.tags, function(t) {
            return t !== kill;
        });
        updateBug("tags");
    }

    $scope.addTags = function($event) {
        //hack because typeahead component breaks angular model;
        $scope.newtag = $("#tagbox").val();
        var newtag = $scope.newtag.split(" ").shift();
        $scope.newtag = '';
        if(!$scope.bug.tags) {
            $scope.bug.tags = [];
        }
        $scope.bug.tags.push(newtag);
        $scope.bug.tags = _.uniq($scope.bug.tags);
        $event.preventDefault();
        updateBug("tags");
    }

    var getTags = function(query, process) {
        $http.get('/api/tags/').success(function(data) {
            var tags = [];
            for (var k in data) {
                tags.push(k);
            }
            tags.sort();
            process(tags);
        });
    }

    $("#tagbox").typeahead({source: getTags});

    $scope.editTitle = function() {
        $scope.editingTitle = true;
    }

    $scope.submitTitle = function() {
        updateBug("title");
        $scope.editingTitle = false;
    }

    $scope.startComment = function() {
        $scope.addingcomment = true;
    }

    $scope.postComment = function() {
        $http.post('/api/bug/' + $routeParams.bugId + '/comments/',
                    'body=' + encodeURIComponent($scope.draftcomment),
                  {headers: {"Content-Type": "application/x-www-form-urlencoded"}}).
            success(function(data) {
                data.mine = true;
                $scope.comments.push(data);
                $scope.draftcomment="";
                $scope.addingcomment = false;
            }).
            error(function(data, code) {
                bAlert("Error " + code, "could not post comment: " + data, "error")
            })
    }

    $scope.deleteComment = function(comment) {
        console.log(comment);
        $http.delete('/api/bug/' + $routeParams.bugId + '/comments/' + comment.id).
            success(function(data) {
                $scope.comments = _.map($scope.comments, function(check) {
                    if(check.id == comment.id) {
                        check.deleted = true;
                    };
                    return check;
                });
            }).
            error(function(data, code) {
                bAlert("Error " + code, "could not post comment: " + data, "error")
            })
    }

    $scope.unDeleteComment = function(comment) {
        console.log(comment);
        $http.post('/api/bug/' + $routeParams.bugId + '/comments/' + comment.id + '/undel').
            success(function(data) {
                $scope.comments = _.map($scope.comments, function(check) {
                    if(check.id == comment.id) {
                        check.deleted = false;
                    };
                    return check;
                });
            }).
            error(function(data, code) {
                bAlert("Error " + code, "could not post comment: " + data, "error")
            })
    }

    var getUsers = function(query, process) {
        console.log("Getting users..");
        $http.get('/api/users/').success(function(data) {
            process(data);
        });
    }

    $scope.editOwner = function() {
        $(".ownerbox").typeahead({source: getUsers});
        $scope.editingowner = true;
    }

    $scope.submitOwner = function() {
        $scope.bug.owner.email = $(".ownerbox").val();
        updateBug("owner", $scope.bug.owner.email);
        $scope.editingowner = false;
    }
}

function FakeLoginCtrl($scope) {
    $rootScope.loginscope = $scope;
    $scope.login = function() {
        $rootScope.loggedin = true;
        $scope.loggedin = true;
        $scope.username = "Test User";
        $scope.gravatar = "eee3b47a26586bb79e0a832466c066be";
    }
    $scope.logout = function() {
        $rootScope.loggedin = false;
        $scope.loggedin = false;
    }
}

function LoginCtrl($scope, $http, $rootScope) {
    $rootScope.loginscope = $scope;
    navigator.id.watch({
        onlogin: function(assertion) {
            $http.post('/auth/login', "assertion="+assertion+"&audience=" +
                encodeURIComponent(location.protocol+"//"+location.host),
                {headers: {"Content-Type": "application/x-www-form-urlencoded"}}).
                success(function(res) {
                    $scope.loggedin = true;
                    $scope.username = res.email;
                    $scope.gravatar = res.emailmd5;
                    $rootScope.loggedin = true;
                }).
                error(function(res, err) {
                    bAlert("Error", "Couldn't log you in.", "error");
                });
        },
        onlogout: function() {
            $rootScope.loggedin = false;
            $scope.loggedin = false;
        }
    });

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
        })
    }

    $scope.login = function() {
        navigator.id.request();
    }
}

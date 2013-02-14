function NavigatorCtrl($scope, $routeParams, $http, $location, cbuggAuth, cbuggPage, cbuggSearch) {

    $scope.result = cbuggSearch.getDefaultSearchResult();
    $scope.result.options = cbuggSearch.getDefaultSearchOptions();
    $scope.auth = cbuggAuth.get();

    $scope.$watch('auth.loggedin', function(newval, oldval) {
        if (!$scope.$eval($scope.defaultTabs[$scope.selectedTab].show)){
            $scope.result.inProgress = false;
            $scope.result.errorMessage = "This view cannot be shown.";
        } else {
            page = $location.search()['page'];
            if(!page) {
                page = 1;
            }
            $scope.jumpToTabPage($scope.selectedTab, page, null);
        }
    });

    $scope.selectedTab = "all";
    if ($routeParams.tab) {
        $scope.selectedTab = $routeParams.tab;
    }

    $scope.defaultTabs = {
        "all" : {
            "show" : "true",
            "name" : "Everyone's Bugs",
            "type" : "elasticsearch",
            "query" : ""
        },
        "assignedToYou" : {
            "show" : "auth.username",
            "name" : "Assigned To You",
            "type" : "elasticsearch",
            "query" : "'owner:'+auth.username"
        },
        "createdByYou" : {
            "show" : "auth.username",
            "name" : "Created By You",
            "type" : "elasticsearch",
            "query" : "'creator:'+auth.username"
        }
    };

    $scope.doSearch = function(options) {
        $scope.result = cbuggSearch.query($scope.query, options);
    };

    $scope.jumpToPage = function(pageNum, $event) {
        $scope.jumpToTabPage($scope.selectedTab, pageNum, $event);
    };

    $scope.jumpToTabPage = function(tab, pageNum, $event) {
        if ($event) {
            $event.preventDefault();
        }

        $scope.selectedTab = tab;
        $scope.query = $scope.$eval($scope.defaultTabs[$scope.selectedTab].query);
        if (!$scope.query) {
            $scope.query = "";
        }

        if($scope.result) {
            options = $scope.result.options;
        } else {
            options = cbuggSearch.getDefaultSearchOptions();
        }
        options.page = pageNum;
        $location.search('page', pageNum);
        $scope.doSearch(options);
    };

    $scope.updateStatusFilter = function(val) {
        pos = $scope.result.options.status.indexOf(val);
        if (pos == -1) {
            $scope.result.options.status.push(val);
        } else {
            $scope.result.options.status.splice(pos, 1);
        }
        $scope.jumpToPage(1, null);
    };

    $scope.updateTagFilter = function(val) {
        pos = $scope.result.options.tags.indexOf(val);
        if (pos == -1) {
            $scope.result.options.tags.push(val);
        } else {
            $scope.result.options.tags.splice(pos, 1);
        }
        $scope.jumpToPage(1, null);
    };

    $scope.updateModifiedFilter = function(val) {
        if ($scope.result.options.last_modified === val) {
            $scope.result.options.last_modified = "";
        } else {
            $scope.result.options.last_modified = val;
        }
        $scope.jumpToPage(1, null);
    };

    $scope.isModifiedFilter = function(val) {
        if ($scope.result.options.last_modified === val) {
            return true;
        } else {
            return false;
        }
    };

    $scope.isSubscribed = function(userhash, subscribers) {
        for (var i in subscribers) {
            subscriber = subscribers[i];
            if (subscriber.md5 == userhash) {
                return true;
            }
        }
        return false;
    };

    cbuggPage.setTitle("Navigator");
}
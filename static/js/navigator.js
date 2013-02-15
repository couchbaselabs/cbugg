function NavigatorCtrl($scope, $routeParams, $http, $location, cbuggAuth, cbuggPage, cbuggSearch) {

    $scope.doSearch = function() {
        $scope.result = cbuggSearch.query($scope.query, $scope.result.options);
        $location.search('page', $scope.result.options.page);
    };

    $scope.jumpToPage = function(pageNum, $event) {
        if ($event) {
            $event.preventDefault();
        }

        $scope.result.options.page = pageNum;
        $scope.doSearch();
    };

    $scope.buildQueryFromTab = function() {
        $scope.query = $scope.$eval($scope.defaultTabs[$scope.selectedTab].query);
        if (!$scope.query) {
            $scope.query = "";
        }
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

    // initialization
    cbuggPage.setTitle("Navigator");
    $scope.result = cbuggSearch.getDefaultSearchResult();
    $scope.result.options = cbuggSearch.getDefaultSearchOptions($location.search());
    $scope.auth = cbuggAuth.get();

    $scope.selectedTab = "all";
    if ($routeParams.tab) {
        $scope.selectedTab = $routeParams.tab;
    }
    $scope.buildQueryFromTab();

    $scope.$watch('auth.loggedin', function(newval, oldval) {
        // if this tab can't be shown, display error
        if (!$scope.$eval($scope.defaultTabs[$scope.selectedTab].show)){
            $scope.result.inProgress = false;
            $scope.result.errorMessage = "This view cannot be shown.";
        } else {
            // need to re-evaluate query (may look different after auth)
            $scope.buildQueryFromTab();
            $scope.doSearch();
        }
    });
}
function NavigatorCtrl($scope, $routeParams, $http, $location, cbuggAuth, cbuggPage, cbuggSearch) {

    $scope.doSearch = function() {
        $scope.result = cbuggSearch.query($scope.query, $scope.result.options);
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

    $scope.updateFilter = function(field, value) {
        $scope.result.options.updateFilter(field, value);
        $scope.jumpToPage(1, null);
    };

    $scope.checkFilter = function(field, value) {
        return $scope.result.options.checkFilter(field, value);
    };

    $scope.sortyBy = function(field) {
        $scope.result.options.sortBy(field);
        $scope.jumpToPage(1, null);
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
    $scope.auth = cbuggAuth.get();
    $scope.result.options = cbuggSearch.getDefaultSearchOptions($location.search(), $scope.auth.prefs);

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
            $scope.result.options = cbuggSearch.getDefaultSearchOptions($location.search(), $scope.auth.prefs);
            $scope.doSearch();
        }
    });
}
function NavigatorCtrl($scope, $routeParams, $http, $rootScope, cbuggAuth, cbuggPage, cbuggSearch) {

    $rootScope.$watch('loggedin', function() {
        $scope.auth = cbuggAuth.get();
        
        if (!$scope.$eval($scope.defaultTabs[$scope.selectedTab].show)){
            $scope.result.searchInProgress = false;
            $scope.result.searchError = "This view cannot be shown.";
        } else {
            $scope.jumpToTabPage($scope.selectedTab, 1, null);
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
        console.log("querying " + $scope.query)
        $scope.result = cbuggSearch.query($scope.query, options);
    }

    $scope.jumpToPage = function(pageNum, $event) {
        $scope.jumpToTabPage($scope.selectedTab, pageNum, $event)
    }

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
            options.page = pageNum;
            $scope.doSearch(options);
        } else {
            $scope.doSearch();
        }
    }

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

    $scope.isSubscribed = function(userhash, subscribers) {
        for (i in subscribers) {
            subscriber = subscribers[i];
            if (subscriber.md5 == userhash) {
                return true;
            }
        }
        return false;
    }

    cbuggPage.setTitle("Navigator");
}
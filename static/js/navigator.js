function NavigatorCtrl($scope, $routeParams, $http, $rootScope, cbuggAuth, cbuggPage) {
    $rootScope.$watch('loggedin', function() {
        $scope.auth = cbuggAuth.get();
        
        if (!$scope.$eval($scope.defaultTabs[$scope.selectedTab].show)){
            $scope.searchInProgress = false;
            $scope.searchError = true;
            $scope.searchWarning = "This view cannot be shown.";
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

    $scope.searchInProgress = true;
    $scope.searchError = false;

    $scope.page = 1;
    if ($routeParams.page) {
        $scope.page = $routeParams.page;
    }

    $scope.rpp = 10;
    $scope.pageSizes = [ 10, 30, 50, 100 ];
    $scope.filterStatus = [];
    $scope.filterTags = [];
    cbuggPage.setTitle("Navigator");
    $scope.maxPagesToShow = 7;

    $scope.jumpToPage = function(pageNum, $event) {
        $scope.jumpToTabPage($scope.selectedTab, pageNum, $event)
    }

    $scope.jumpToTabPage = function(tab, pageNum, $event) {
        if ($event != null) {
            $event.preventDefault();
        }
        $scope.selectedTab = tab;
        $scope.query = $scope
                .$eval($scope.defaultTabs[$scope.selectedTab].query);
        if (!$scope.query) {
            $scope.query = "";
        }
        $scope.page = pageNum;
        $http.post(
            '/api/search/?query=' + $scope.query + '&from='
                    + (($scope.page - 1) * $scope.rpp) + '&size=' + $scope.rpp
                    + "&status=" + $scope.filterStatus.join(",") + "&tags="
                    + $scope.filterTags.join(",")).success(function(data) {
            $scope.searchWarning = null;
            $scope.searchError = false;
            $scope.searchInProgress = false;
            $scope.shards = data._shards;
            $scope.results = data.hits.hits;
            $scope.facets = data.facets;
            $scope.total = data.hits.total;
            $scope.computeValidPages();
            $scope.verifyAllSearchShards();
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
        // compute the valid pages
        $scope.numPages = Math.ceil($scope.total / $scope.rpp);
        $scope.validPages = new Array();
        for (i = 1; i <= $scope.numPages; i++) {
            $scope.validPages.push(i);
        }

        // now see if we have too many pages
        if ($scope.validPages.length > $scope.maxPagesToShow) {
            numPagesToRemove = $scope.validPages.length - $scope.maxPagesToShow;
            frontPagesToRemove = backPagesToRemove = 0;
            while (numPagesToRemove - frontPagesToRemove - backPagesToRemove > 0) {
                numPagesBefore = $scope.page - 1 - frontPagesToRemove;
                numPagesAfter = $scope.validPages.length - $scope.page
                        - backPagesToRemove;
                if (numPagesAfter > numPagesBefore) {
                    backPagesToRemove++;
                } else {
                    frontPagesToRemove++;
                }
            }

            // remove from the end first, to keep indexes simpler
            $scope.validPages.splice(-backPagesToRemove, backPagesToRemove);
            $scope.validPages.splice(0, frontPagesToRemove);
        }

        // now compute the first and last result shown on this page
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
        $scope.jumpToTabPage($scope.selectedTab, 1, null);
    }

    $scope.updateTagFilter = function(val) {
        pos = $scope.filterTags.indexOf(val);
        if (pos == -1) {
            $scope.filterTags.push(val)
        } else {
            $scope.filterTags.splice(pos, 1);
        }
        $scope.jumpToTabPage($scope.selectedTab, 1, null);
    }

    $scope.changeRpp = function(size, $event) {
        $scope.rpp = size;
        $scope.jumpToTabPage($scope.selectedTab, 1, $event);
    }

    $scope.isSubscribed = function(userhash, subscribers) {

        for (i in subscribers) {
            subscriber = subscribers[i];
            if (subscriber.md5 == userhash) {
                return true;
            }
        }

        return false;
    }

}
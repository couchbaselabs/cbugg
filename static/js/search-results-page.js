function SearchResultsCtrl($scope, $routeParams, $http, $rootScope, cbuggAuth, cbuggPage) {

    $rootScope.$watch('loggedin', function() {
        $scope.auth = cbuggAuth.get();
    });

    $scope.searchInProgress = true;
    $scope.searchError = false;
    $scope.page = 1;
    $scope.rpp = 10;
    $scope.pageSizes = [ 10, 30, 50, 100 ];
    $scope.filterStatus = [];
    $scope.filterTags = [];
    cbuggPage.setTitle("Search");

    $scope.jumpToPage = function(pageNum, $event) {
        if ($event) {
            $event.preventDefault();
        }
        $scope.page = pageNum;
        $http.post(
            '/api/search/?query=' + $routeParams.query + '&from=' +
                    (($scope.page - 1) * $scope.rpp) + '&size=' + $scope.rpp +
                    "&status=" + $scope.filterStatus.join(",") + "&tags=" +
                    $scope.filterTags.join(",")).success(function(data) {
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
            $scope.searchWarning = "Search only contains results from " +
                    $scope.shards.successful + " of " + $scope.shards.total +
                    " shards";
        }
    };

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
            $scope.filterStatus.push(val);
        } else {
            $scope.filterStatus.splice(pos, 1);
        }
        $scope.jumpToPage(1, null);
    };

    $scope.updateTagFilter = function(val) {
        pos = $scope.filterTags.indexOf(val);
        if (pos == -1) {
            $scope.filterTags.push(val);
        } else {
            $scope.filterTags.splice(pos, 1);
        }
        $scope.jumpToPage(1, null);
    };

    $scope.changeRpp = function(size, $event) {
        $scope.rpp = size;
        $scope.jumpToPage(1, $event);
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

    $scope.query = $routeParams.query;
    $scope.jumpToPage(1, null);

}
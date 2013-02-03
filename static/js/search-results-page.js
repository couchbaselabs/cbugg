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
        $scope.numPages = Math.ceil($scope.total / $scope.rpp);
        $scope.validPages = [];
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

    $scope.query = $routeParams.query;
    $scope.jumpToPage(1, null);

}
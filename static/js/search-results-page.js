function SearchResultsCtrl($scope, $routeParams, $location, cbuggPage, cbuggSearch) {

    $scope.doSearch = function() {
        $scope.result = cbuggSearch.query($routeParams.query, $scope.result.options);
        $location.search('page', $scope.result.options.page);
    };

    $scope.jumpToPage = function(pageNum, $event) {
        if ($event) {
            $event.preventDefault();
        }

        $scope.result.options.page = pageNum;
        $scope.doSearch();
    };

    $scope.updateFilter = function(field, value) {
        $scope.result.options.updateFilter(field, value);
        $scope.jumpToPage(1, null);
    };

    $scope.checkFilter = function(field, value) {
        return $scope.result.options.checkFilter(field, value);
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

    cbuggPage.setTitle("Search");
    $scope.result = cbuggSearch.getDefaultSearchResult();
    $scope.result.options = cbuggSearch.getDefaultSearchOptions($location.search());
    $scope.doSearch();
}
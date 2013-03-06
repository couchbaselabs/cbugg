function SearchResultsCtrl($scope, $routeParams, $location, cbuggPage, cbuggSearch, cbuggAuth) {

    $scope.doSearch = function() {
        $scope.result = cbuggSearch.query($routeParams.query, $scope.result.options);
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
    $scope.auth = cbuggAuth.get();

    $scope.$watch('auth.loggedin', function(newval, oldval) {
        // need to re-evaluate query (may look different after auth)
        $scope.result = cbuggSearch.getDefaultSearchResult();
        $scope.result.options = cbuggSearch.getDefaultSearchOptions($location.search(), $scope.auth.prefs);
        $scope.doSearch();
    });
}
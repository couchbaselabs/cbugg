function SearchResultsCtrl($scope, $routeParams, cbuggPage, cbuggSearch) {

    $scope.doSearch = function(options) {
        $scope.result = cbuggSearch.query($routeParams.query, options);
    }

    $scope.jumpToPage = function(pageNum, $event) {
        if ($event) {
            $event.preventDefault();
        }

        options = $scope.result.options;
        options.page = pageNum;
        $scope.doSearch(options);
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

    cbuggPage.setTitle("Search");
    $scope.doSearch();

}
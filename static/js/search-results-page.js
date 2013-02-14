function SearchResultsCtrl($scope, $routeParams, $location, cbuggPage, cbuggSearch) {

    $scope.doSearch = function(options) {
        $scope.result = cbuggSearch.query($routeParams.query, options);
    };

    $scope.jumpToPage = function(pageNum, $event) {
        if ($event) {
            $event.preventDefault();
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

    cbuggPage.setTitle("Search");
    // look at the request and see what page they requested
    page = $location.search()['page'];
    if(!page) {
        page = 1;
    }
    $scope.jumpToPage(page, null);

}
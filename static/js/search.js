var cbuggSearch = angular.module('cbuggSearch', []);

cbuggSearch.factory('cbuggSearch', function($http) {

	var newPager = function(currentPage, totalResults, resultsPerPage, maxPagesToShow) {
		numPages = Math.ceil(totalResults / resultsPerPage);
		validPages = new Array();

        for (i = 1; i <= numPages; i++) {
            validPages.push(i);
        }


        // now see if we have too many pages
        if (validPages.length > maxPagesToShow) {
            numPagesToRemove = validPages.length - maxPagesToShow;
            frontPagesToRemove = backPagesToRemove = 0;
            while (numPagesToRemove - frontPagesToRemove - backPagesToRemove > 0) {
                numPagesBefore = currentPage - 1 - frontPagesToRemove;
                numPagesAfter = validPages.length - currentPage
                        - backPagesToRemove;
                if (numPagesAfter > numPagesBefore) {
                    backPagesToRemove++;
                } else {
                    frontPagesToRemove++;
                }
            }

            // remove from the end first, to keep indexes simpler
            validPages.splice(-backPagesToRemove, backPagesToRemove);
            validPages.splice(0, frontPagesToRemove);
        }

        // now compute the first and last result shown on this page
        firstResult = ((currentPage - 1) * resultsPerPage) + 1;
        if (firstResult > totalResults) {
            firstResult = totalResults;
        }
        lastResult = firstResult + resultsPerPage - 1;
        if (lastResult > totalResults) {
            lastResult = totalResults;
        }

		return {
			"currentPage": currentPage,
			"numPages": numPages,
			"resultsPerPage": resultsPerPage,
			"validPages": validPages,
			"firstResult": firstResult,
			"lastResult": lastResult,
			"totalResults": totalResults
		};
	}

	return {
		query: function(query_string, options) {
			console.log("starting search query")
			options = (typeof options !== "undefined") ? options : {
				"page": 1,
				"rpp": 10,
				"status": [],
				"tags": [],
				"maxPagesToShow": 7
			};

			query = '/api/search/' 
			+ '?query=' + query_string
			+ '&from=' + (options.page - 1) * options.rpp 
			+ '&size=' + options.rpp
			+ '&status=' + options.status.join(',')
			+ '&tags=' + options.tags.join(',');

			result = {
				inProgress: true,
				errorMessage: "",
				warningMessage: "",
				query_string: query_string,
				options: options,
				hits: [],
				facets: {},
				pager: {}
			};

			$http.post(query).success(function (data) {
				result.hits = data.hits.hits;
				result.facets = data.facets;

				//build pager
				result.pager = newPager(options.page, data.hits.total, options.rpp, 
					options.maxPagesToShow);

				// check results came from all shards
				if(data._shards.total !== data._shards.successful) {
					result.warningMessage = "Search only contains results from "
					+ data._shards.successful + " of " + data._shards.total + "shards"
				}

				result.inProgress = false;
			}).error(function(data, status, headers, config) {
				result.errorMessage = data;
				result.inProgress = false;
			});

			return result;
		},
	}

});
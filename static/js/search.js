var cbuggSearch = angular.module('cbuggSearch', []);

cbuggSearch.factory('cbuggSearch', function($http) {

	var newPager = function(currentPage, totalResults, resultsPerPage, maxPagesToShow) {
		numPages = Math.ceil(totalResults / resultsPerPage);
		validPages = [];

        for (i = 1; i <= numPages; i++) {
            validPages.push(i);
        }


        // now see if we have too many pages
        if (validPages.length > maxPagesToShow) {
            numPagesToRemove = validPages.length - maxPagesToShow;
            frontPagesToRemove = backPagesToRemove = 0;
            while (numPagesToRemove - frontPagesToRemove - backPagesToRemove > 0) {
                numPagesBefore = currentPage - 1 - frontPagesToRemove;
                numPagesAfter = validPages.length - currentPage - backPagesToRemove;
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
	};

	var defaultSearchOptions = function(overrides) {
		defaultPage = (overrides && overrides["page"]) ? overrides["page"] : 1;
		return {
			"page": defaultPage,
			"rpp": 10,
			"status": [],
			"tags": [],
			"last_modified": "",
			"maxPagesToShow": 7,
			updateFilter: function(field, value) {
				switch(field) {
					case "status":
					case "tags":
						this.updateMultipleValueFilter(field,value);
					break;
					case "last_modified":
						this.updateSingleValueFilter(field, value);
					break;
				}
			},
			updateMultipleValueFilter: function(field, value) {
				pos = this[field].indexOf(value);
				if (pos === -1) {
					this[field].push(value);
				} else {
					this[field].splice(pos, 1);
				}
			},
			updateSingleValueFilter: function(field, value) {
				if (this[field] === value) {
					this[field] = "";
				} else {
					this[field] = value;
				}
			},
			checkFilter: function(field, value) {
				switch(field) {
					case "status":
					case "tags":
						return this.checkMultipleValueFilter(field,value);
					case "last_modified":
						return this.checkSingleValueFilter(field, value);
				}
			},
			checkMultipleValueFilter: function(field, value) {
				return this[field].indexOf(value) !== -1;
			},
			checkSingleValueFilter: function(field, value) {
				return this[field] === value;
			}
		};
	};

	var defaultSearchResult = function() {
		return {
			inProgress: true,
			errorMessage: "",
			warningMessage: "",
			query_string: "",
			options: {},
			hits: [],
			facets: {},
			pager: {}
		};
	};

	return {
		getDefaultSearchOptions: function(overrides) {
			return defaultSearchOptions(overrides);
		},
		getDefaultSearchResult: function() {
			return defaultSearchResult();
		},
		query: function(query_string, options) {
			options = (typeof options !== "undefined") ? options : defaultSearchOptions();

			query = '/api/search/' +
			'?query=' + query_string +
			'&from=' + (options.page - 1) * options.rpp +
			'&size=' + options.rpp +
			'&status=' + options.status.join(',') +
			'&tags=' + options.tags.join(',') +
			'&modified=' + options.last_modified;

			result = defaultSearchResult();
			result.query_string = query_string;
			result.options = options;

			$http.post(query).success(function (data) {
				result.hits = data.hits.hits;
				result.facets = data.facets;

				//build pager
				result.pager = newPager(options.page, data.hits.total, options.rpp, options.maxPagesToShow);

				// check results came from all shards
				if(data._shards.total !== data._shards.successful) {
					result.warningMessage = "Search only contains results from " +
					data._shards.successful + " of " + data._shards.total + " shards";
				}

				result.inProgress = false;
			}).error(function(data, status, headers, config) {
				result.errorMessage = data;
				result.inProgress = false;
			});

			return result;
		}
	};

});
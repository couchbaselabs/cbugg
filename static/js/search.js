var cbuggSearch = angular.module('cbuggSearch', []);

cbuggSearch.factory('cbuggSearch', function($http, $location) {

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

	var defaultSearchOptions = function(customOptions, prefs) {
		var defaultPage = 1;
		var defaultStatus = [];
		var defaultTags = [];
		var defaultLastModified = "";
		var defaultRpp = 15;
		var defaultSort = "-_score";
		if(customOptions) {
			for(var option in customOptions) {
				var value = customOptions[option];
				if(option == "page") {
					defaultPage = value;
				} else if (option == "status") {
					var mv = value;
					if(typeof value === 'string') {
						mv = value.split(",");
					}
					for(var i in mv) {
						defaultStatus.push(mv[i]);
					}
				} else if (option == "tags") {
					var mv2 = value;
					if(typeof value === 'string') {
						mv2 = value.split(",");
					}
					for(var i2 in mv2) {
						defaultTags.push(mv2[i2]);
					}
				} else if (option == "last_modified") {
					defaultLastModified = value;
				} else if (option == "sort") {
					defaultSort = value;
				}
			}
		}
		if(prefs && prefs.search && prefs.search.rowsPerPage) {
			defaultRpp = parseInt(prefs.search.rowsPerPage, 10);
		}
		return {
			"page": defaultPage,
			"rpp": defaultRpp,
			"status": defaultStatus,
			"tags": defaultTags,
			"last_modified": defaultLastModified,
			"maxPagesToShow": 7,
			"sort": defaultSort,
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
			},
			sortBy: function(field) {
				if(this.sort.match(field + "$")) {
					// same field, just toggle asc/desc
					if(this.sort[0] == "-") {
						this.sort = field;
					} else {
						this.sort = "-" + field;
					}
				} else {
					// sort is changing field, default to desc
					this.sort = "-" + field;
				}
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

	var updateLocationWithOptions = function(options) {
		$location.search('page', options.page);
        if(options.status.length > 0) {
            $location.search('status', options.status);
        } else {
            $location.search('status', null);
        }
        if(options.tags.length > 0) {
            $location.search('tags', options.tags);
        } else {
            $location.search('tags', null);
        }
        if(options.last_modified) {
            $location.search('last_modified', options.last_modified);
        } else {
            $location.search('last_modified', null);
        }
        if(options.sort) {
			$location.search('sort', options.sort);
        } else {
			$location.search('sort', null);
        }
	};

	return {
		getDefaultSearchOptions: function(overrides, prefs) {
			return defaultSearchOptions(overrides, prefs);
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
			'&modified=' + options.last_modified +
			'&sort=' + options.sort;

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

				//update the location
				updateLocationWithOptions(result.options);
			}).error(function(data, status, headers, config) {
				result.errorMessage = data;
				result.inProgress = false;
			});

			return result;
		}
	};

});
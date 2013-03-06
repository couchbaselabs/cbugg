var cbuggPrefs = angular.module('cbuggPrefs', []);
cbuggPrefs.factory('cbuggPrefs', function($http) {

	function getDefaultPreferences() {
		return {
			search: {
				rowsPerPage: 15
			}
		};
	}

	function getUserPreferences() {
		var result = getDefaultPreferences();
		$http.get('/api/me/').
			success(function(res) {
				var userPrefs = res.prefs;
				//overlay the user preferences over the defaults
				$.extend(true, result, userPrefs);
			});
		return result;
	}

	function saveUserPreferences(prefs) {
		$http.post('/api/me/prefs/', prefs).
			error(function(err) {
				console.log("Error saving preferences");
				console.log(err);
			});
	}

	return {
		getDefaultPreferences: getDefaultPreferences,
		getUserPreferences: getUserPreferences,
		saveUserPreferences: saveUserPreferences
	};

});

function PrefsCtrl($scope, $http, cbuggAuth, cbuggPage, cbuggPrefs) {

	cbuggPage.setTitle("Preferences");
	$scope.auth = cbuggAuth.get();

	$scope.save = function() {
		cbuggPrefs.saveUserPreferences($scope.auth.prefs);
	};

}
var cbuggPage = angular.module('cbuggPage', []);

cbuggPage.factory('cbuggPage', function($rootScope) {
    
    return {
      setTitle: function(title) { $rootScope.title = title; }
    };
    
});
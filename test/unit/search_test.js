var successfulEmtpySearchResponse = {
    "hits": {
        "hits": [],
        "total": 0
    },
    "_shards": {
        "total": 5,
        "successful": 5
    }
};

var partiallySuccessfulEmtpySearchResponse = {
    "hits": {
        "hits": [],
        "total": 0
    },
    "_shards": {
        "total": 5,
        "successful": 4
    }
};

describe("Search Service", function() {

	var service, $httpBackend;

	beforeEach(module('cbuggSearch'));
	beforeEach(inject(function(cbuggSearch, _$httpBackend_) {
		service = cbuggSearch;
		$httpBackend = _$httpBackend_;
	}));


    it('should invoke service with right paramaeters', function() {
        $httpBackend.expectPOST('/api/search/?query=bug&from=0&size=15&status=&tags=&modified=&sort=-_score').respond(successfulEmtpySearchResponse);
        result = service.query("bug");
        $httpBackend.flush();
        expect(result.inProgress).toBe(false);
        expect(result.errorMessage).toBe("");
        expect(result.warningMessage).toBe("");
        expect(result.hits.length).toBe(0);
    });

    it('should report a warning when response is not from all shards', function() {
        $httpBackend.expectPOST('/api/search/?query=bug&from=0&size=15&status=&tags=&modified=&sort=-_score').respond(partiallySuccessfulEmtpySearchResponse);
        result = service.query("bug");
        $httpBackend.flush();
        expect(result.inProgress).toBe(false);
        expect(result.errorMessage).toBe("");
        expect(result.warningMessage).toBe("Search only contains results from 4 of 5 shards");
        expect(result.hits.length).toBe(0);
    });

    it('should report an error when search API returns an error', function() {
        $httpBackend.expectPOST('/api/search/?query=bug&from=0&size=15&status=&tags=&modified=&sort=-_score').respond(500, 'dial tcp 127.0.0.1:9200: connection refused');
        service.query("bug");
        $httpBackend.flush();
        expect(result.inProgress).toBe(false);
        expect(result.errorMessage).toBe("dial tcp 127.0.0.1:9200: connection refused");
    });

    afterEach(function() {
        $httpBackend.verifyNoOutstandingExpectation();
        $httpBackend.verifyNoOutstandingRequest();
    });


});
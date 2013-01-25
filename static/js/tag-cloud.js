function cloudify(tags) {
    tags = _.pairs(tags);

    tags.sort(function(a, b) { return b[1] - a[1]; });
    tags = _.first(tags, 75);

    function sumOf(a) {
        var sum = 0;
        for (var i = 0; i < a.length; ++i) {
            sum += a[i][1];
        }
        return sum;
    }

    var sum = sumOf(tags);
    var each = sum / tags.length;

    var clumps = [];
    for(var i = 0; i < 5; ++i) {
        clumps[i] = [];
    }

    var current = 0;
    _.each(tags, function(tag) {
        if (sumOf(clumps[current]) + tag[1] > each) {
            ++current;
        }
        if (current >= clumps.length) {
            current = clumps.length - 1;
        }
        clumps[current].push(tag);
    });

    tags = [];

    _.each(clumps, function(clump, i) {
        _.each(clump, function(tag) {
            tags.push({key: tag[0],
                       count: tag[1],
                       weight: i});
        });
    });

    return _.sortBy(tags, 'key');
}

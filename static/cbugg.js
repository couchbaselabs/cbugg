function cbuggFormToJSON(selector, defaults) {
    var form = defaults;
    $(selector).find(':input[name]:enabled').each(function() {
        var self = $(this);
        var name = self.attr('name');
        if (form[name]) {
            form[name] = form[name] + ',' + self.val();
        } else {
            form[name] = self.val();
        }
    });

    return form;
}

if (!Date.prototype.toISOString) {
    Date.prototype.toISOString = function() {
        function pad(n) { return n < 10 ? '0' + n : n; }
        return this.getUTCFullYear() + '-'
            + pad(this.getUTCMonth() + 1) + '-'
            + pad(this.getUTCDate()) + 'T'
            + pad(this.getUTCHours()) + ':'
            + pad(this.getUTCMinutes()) + ':'
            + pad(this.getUTCSeconds()) + 'Z';
    };
}

function cbuggNewID() {
    return "bug-" + new Date().toISOString();
}

function cbuggNewBug() {
    var data = JSON.stringify(cbuggFormToJSON("#newbug", {
        description: "",
        status: "new",
        creator: "me",
        tags: [],
        type: "bug",
        created_at: new Date().toISOString()
    }));

    var newId = cbuggNewID();

    $.ajax({
        type: "PUT",
        url: "/.cbfs/crudproxy/" + newId,
        contentType: "application/json",
        data: data,
        success: function(data) {
            alert('Bug submitted!');
        }
    });

    return false;
}

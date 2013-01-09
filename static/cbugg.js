function  cbuggMakePageEditable(buginfo) {
    var url = '/bug/' + buginfo.bugid;
    $('.edit').editable(url);

    $('#status').editable(url, {
        type: 'select',
        submit: 'OK',
        data: {'new': 'new', 'open': 'open', 'resolved': 'resolved',
               'closed': 'closed', 'selected': buginfo.status }
    });

    $('#tags').editable(url, {
        data: function(value, settings) {
            var val = $.trim(value);
            if (val === '<em>Untagged</em>') {
                val = '';
            }
            return val.replace(/\s+/g, ' ');
        }
    });

    $('.edit_area').editable(url, {
        type: 'textarea',
        cancel: 'Cancel',
        submit: 'OK',
        width: "none",
        height: "none",
        tooltip: 'click to modify description'
    });
}

function  cbuggMakePageEditable(buginfo) {
    var url = '/bug/' + buginfo.bugid;
    $('.edit').editable(url);

    $('#status').editable(url, {
        type: 'select',
        submit: 'OK',
        data: {'new': 'new', 'open': 'open', 'resolved': 'resolved',
               'closed': 'closed', 'selected': buginfo.status }
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

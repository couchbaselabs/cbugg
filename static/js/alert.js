var cbuggAlert = angular.module('cbuggAlert', []);
cbuggAlert.factory('bAlert', function() {
    function bAlert(heading, message, kind) {
        var kindclass = "";
        if(kind) {
            kindclass = "alert-" + kind;
        }
        $(".app").prepend(
            "<div class='alert fade in " + kindclass + "'>"+
            "<button type='button' class='close' data-dismiss='alert'>&times;</button>"+
            "<strong>" + heading + ":</strong> " + message + "</div>");
        $(".alert").alert();
    }
    return bAlert;
});

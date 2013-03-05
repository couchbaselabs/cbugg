function BugCtrl($scope, $routeParams, $http, $rootScope, $timeout, $location, bAlert, cbuggAuth, cbuggPage, cbuggGrowl) {
    $rootScope.$watch('loggedin', function() { $scope.auth = cbuggAuth.get(); });
    var updateBug = function(field, newValue) {
        var bug = $scope.bug;
        if (newValue === undefined) {
            newValue = bug[field];
        }
        if(bug && newValue) {
            $http.post('/api/bug/' + bug.id, "id=" + field + "&value=" + encodeURIComponent(newValue),
                       {headers: {"Content-Type": "application/x-www-form-urlencoded"}}).
            success(function(data) {
                $scope.bug = data;
                checkSubscribed();
            }).
            error(function(data, code) {
                bAlert("Error " + code, "could not update bug: " + data, "error");
            });
        }
    };

    $scope.availableStates = [];
    $scope.comments = [];
    $scope.attachments = [];
    $scope.draftcomment = "";
    $scope.draftcommentpriv = false;
    $scope.subscribed = false;
    $scope.currentuser = null;
    $scope.privateclass = "";

    $scope.allStates = $http.get("/api/states/").then(function(resp) {
        return resp.data;
    });

    // ============== DRAG & DROP =============
    // http://www.webappers.com/2011/09/28/drag-drop-file-upload-with-html5-javascript/
    var dropbox = document.getElementById("dropbox");
    $scope.dropText = 'Drop files here...';

    // init event handlers
    function dragEnterLeave(evt) {
        evt.stopPropagation();
        evt.preventDefault();
        $scope.$apply(function(){
            $scope.dropText = 'Drop files here...';
            $scope.dropClass = '';
        });
    }
    dropbox.addEventListener("dragenter", dragEnterLeave, false);
    dropbox.addEventListener("dragleave", dragEnterLeave, false);
    dropbox.addEventListener("dragover", function(evt) {
        evt.stopPropagation();
        evt.preventDefault();
        var clazz = 'not-available';
        var ok = evt.dataTransfer && evt.dataTransfer.types && evt.dataTransfer.types.indexOf('Files') >= 0;
        $scope.$apply(function(){
            $scope.dropText = ok ? 'Drop files here...' : 'Only files are allowed!';
            $scope.dropClass = ok ? 'over' : 'not-available';
        });
    }, false);
    dropbox.addEventListener("drop", function(evt) {
        console.log('drop evt:', JSON.parse(JSON.stringify(evt.dataTransfer)));
        evt.stopPropagation();
        evt.preventDefault();
        $scope.$apply(function(){
            $scope.dropText = 'Drop files here...';
            $scope.dropClass = '';
        });
        var files = evt.dataTransfer.files;
        if (files.length > 0) {
            $scope.$apply(function(){
                $scope.files = [];
                for (var i = 0; i < files.length; i++) {
                    $scope.files.push(files[i]);
                }
            });
            $scope.uploadFile();
        }
    }, false);
    // ============== DRAG & DROP =============

    $scope.uploadFile = function() {
        var fd = new FormData();
        if ($scope.files.length > 0) {
            fd.append("uploadedFile", $scope.files[0]);
        } else {
            return;
        }
        var xhr = new XMLHttpRequest();
        xhr.upload.addEventListener("progress", uploadProgress, false);
        xhr.addEventListener("load", uploadComplete, false);
        xhr.addEventListener("error", uploadFailed, false);
        xhr.addEventListener("abort", uploadCanceled, false);
        xhr.open("POST", "/api/bug/" + $scope.bug.id + "/attachments/");
        $scope.progressVisible = true;
        xhr.send(fd);
    };

    function uploadProgress(evt) {
        $scope.$apply(function(){
            if (evt.lengthComputable) {
                $scope.progress = Math.round(evt.loaded * 100 / evt.total);
            } else {
                $scope.progress = 'unable to compute';
            }
        });
    }

    function uploadComplete(evt) {
        var j = JSON.parse(evt.currentTarget.responseText);
        $scope.progressVisible = false;
        $scope.files = _.filter($scope.files,
                                function(e) {
                                    return e.name != j.filename;
                                });
        j.mine = true;
        $scope.attachments.push(j);
        $scope.uploadFile();
        $scope.$apply();
    }

    function uploadFailed(evt) {
        alert("There was an error attempting to upload the file.");
    }

    function uploadCanceled(evt) {
        $scope.$apply(function(){
            $scope.progressVisible = false;
        });
        alert("The upload has been canceled by the user or the browser dropped the connection.");
    }

    var updateAvailableStates = function(current) {
        $scope.allStates.then(function(allStates) {
            var scopeMap = _.object(_.pluck(allStates, 'name'), allStates);

            $scope.availableStates = [current];
            var cur = scopeMap[current];
            var targets = cur.targets ||  _.without(_.keys(scopeMap), current);
            $scope.availableStates = $scope.availableStates.concat(targets);

            $scope.availableStates = _.sortBy($scope.availableStates,
                                              function(e) {
                                                  return scopeMap[e].order;
                                              });
        });
    };

    cbuggPage.setTitle($routeParams.bugId);

    $http.get('/api/bug/' + $routeParams.bugId).success(function(data) {
        $scope.bug = data;

        $scope.$watch('bug.private', function(next, prev) {
            $scope.privateclass = next ? "private" : "";
            if ($scope.bug && next !== prev) {
                updateBug("private", "" + next);
            }
        });

        $scope.subcount = data.subscribers.length;
        $scope.$watch('bug.status', function(next, prev) {
            if(next) { updateAvailableStates(next); }
            if($scope.bug && next !== prev) {
                updateBug("status");
            }
        });
        if(!$scope.bug.description) {
            $scope.editDesc();
        }
        checkSubscribed();
    }).
        error(function(data, code) {
            bAlert("Error " + code, "could not fetch bug info: " + data, "error");
        });

    $scope.saveDesc = function() {
        updateBug("description");
        $timeout(function() { $scope.editingDesc = false; });
    };


    $http.get("/api/bug/" + $routeParams.bugId + "/history/").success(function(data) {
        $scope.history = data;
        $scope.history.reverse();
    });

    var checkSubscribed = function() {
        if($scope.bug) {
            $scope.subcount = $scope.bug.subscribers.length;
            _.forEach($scope.bug.subscribers, function(el) {
                $scope.subscribed |= ($scope.auth.gravatar == el.md5);
            });
        }
    };

    var checkOwnership = function (objects) {
        var owned = _.map(objects, function(ob) {
            if(ob.user && $rootScope.loggedin && $scope.auth.gravatar == ob.user.md5) {
                ob.mine = true;
            } else {
                ob.mine = false;
            }
            if ($scope.currentuser && $scope.currentuser.admin) {
                ob.mine = true;
            }
            return ob;
        });
        return _.filter(owned, function(ob) {
            return ob.mine || !ob.deleted;
        });
    };

    $rootScope.$watch("loggedin", function(nv, ov) {
        // Update delete button available on loggedinnness change
        $scope.comments = checkOwnership($scope.comments);
        $scope.attachments = checkOwnership($scope.attachments);
        $(".newuserbox").typeahead({source: $scope.getUsers});
        checkSubscribed();
        $http.get("/api/me/").success(function(data) {
            $scope.currentuser = data;
            if (data.admin) {
                $scope.comments = checkOwnership($scope.comments);
                $scope.attachments = checkOwnership($scope.attachments);
            }
        });
    });

    $http.get('/api/bug/' + $routeParams.bugId + '/comments/').success(function(data) {
        $scope.comments = checkOwnership(data);
    });

    $http.get('/api/bug/' + $routeParams.bugId + '/attachments/').success(function(data) {
        $scope.attachments = checkOwnership(data);
    });

    $scope.killTag = function(kill) {
        $scope.bug.tags = _.filter($scope.bug.tags, function(t) {
            return t !== kill;
        });
        updateBug("tags");
    };

    $scope.addTags = function($event) {
        // hack because typeahead component breaks angular model;
        $scope.newtag = $("#tagbox").val();
        var newtag = $scope.newtag.split(" ").shift();
        $scope.newtag = '';
        if(!$scope.bug.tags) {
            $scope.bug.tags = [];
        }
        $scope.bug.tags.push(newtag);
        $scope.bug.tags = _.uniq($scope.bug.tags);
        if($event) {
            $event.preventDefault();
        }
        updateBug("tags");
    };

    var getTags = function(query, process) {
        $http.get('/api/tags/').success(function(data) {
            var tags = [];
            for (var k in data) {
                tags.push(k);
            }
            tags.sort();
            process(_.difference(tags, $scope.bug.tags));
        });
    };

    $("#tagbox").typeahead({
        source: getTags,
        updater: function(i) {
            $timeout($scope.addTags);
            return i;
        }
    });

    $scope.subscribe = function() {
        $http.post('/api/bug/' + $scope.bug.id + '/sub/');
        $scope.subscribed = true;
        $scope.subcount++;
        return false;
    };

    $scope.delete_bug = function() {
        if (confirm("Are you sure you want to completely and irreversibly destroy this bug?")) {
            $http.delete('/api/bug/' + $scope.bug.id);
            $location.path("/");
        }
        return false;
    };

    $scope.unsubscribe = function() {
        $http.delete('/api/bug/' + $scope.bug.id + '/sub/');
        $scope.subscribed = false;
        $scope.subcount--;
        return false;
    };

    $scope.editTitle = function() {
        $scope.editingTitle = true;
    };

    $scope.submitTitle = function() {
        updateBug("title");
        $scope.editingTitle = false;
    };

    $scope.startComment = function() {
        $scope.addingcomment = true;
        $scope.editComment();
    };

    $scope.postComment = function() {
        $http.post('/api/bug/' + $routeParams.bugId + '/comments/',
                    'body=' + encodeURIComponent($scope.draftcomment) +
                   '&private=' + $scope.draftcommentpriv,
                  {headers: {"Content-Type": "application/x-www-form-urlencoded"}}).
            success(function(data) {
                console.log("Posting comment", $scope.draftcomment, $scope.draftcommentpriv);
                data.mine = true;
                $scope.comments.push(data);
                $scope.draftcomment="";
                $scope.draftcommentpriv=false;
                $timeout(function() { $scope.addingcomment = false; });
                if (!$scope.subscribed) {
                    $scope.subcount++;
                    $scope.subscribed = true;
                }
            }).
            error(function(data, code) {
                bAlert("Error " + code, "could not post comment: " + data, "error");
            });
    };

    $scope.deleteAttachment = function(att) {
        $http.delete('/api/bug/' + $routeParams.bugId + '/attachments/' + att.id + "/").
            success(function(data) {
                $scope.attachments = _.filter($scope.attachments, function(check) {
                    return check.id != att.id;
                });
            }).
            error(function(data, code) {
                bAlert("Error " + code, "could not delete attachment: " + data, "error");
            });
    };

    $scope.deleteComment = function(comment) {
        $http.delete('/api/bug/' + $routeParams.bugId + '/comments/' + comment.id).
            success(function(data) {
                $scope.comments = _.map($scope.comments, function(check) {
                    if(check.id == comment.id) {
                        check.deleted = true;
                    }
                    return check;
                });
            }).
            error(function(data, code) {
                bAlert("Error " + code, "could not post comment: " + data, "error");
            });
    };

    $scope.unDeleteComment = function(comment) {
        $http.post('/api/bug/' + $routeParams.bugId + '/comments/' + comment.id + '/undel').
            success(function(data) {
                $scope.comments = _.map($scope.comments, function(check) {
                    if(check.id == comment.id) {
                        check.deleted = false;
                    }
                    return check;
                });
            }).
            error(function(data, code) {
                bAlert("Error " + code, "could not post comment: " + data, "error");
            });
    };

    $scope.getUsers = function(query, process) {
        $http.get('/api/users/').success(function(data) {
            process(data);
        });
    };

    $scope.editOwner = function() {
        if ($scope.auth && $scope.auth.loggedin) {
            $(".ownerbox").typeahead({source: $scope.getUsers});
            $scope.editingowner = true;
        }
    };

    $scope.setOwnerToMe = function() {
        $scope.bug.owner.email = $scope.auth.username;
        updateBug("owner", $scope.bug.owner.email);
        $scope.editingowner = false;
        return false;
    };

    $scope.listTag = function(tagname) {
        $location.path("/tag/" + encodeURIComponent(tagname));
        return false;
    };

    $scope.submitOwner = function() {
        $scope.bug.owner.email = $(".ownerbox").val();
        updateBug("owner", $scope.bug.owner.email);
        $scope.editingowner = false;
    };

    $scope.rmSpecial = function(u) {
        $http.post('/api/bug/' + $routeParams.bugId + '/viewer/remove/',
                   'email=' + u.md5,
                  {headers: {"Content-Type": "application/x-www-form-urlencoded"}}).
            success(function(data) {
                $scope.bug.also_visible_to = data.also_visible_to;
            }).
            error(function(data, code) {
                bAlert("Error " + code, "could not update visibility", "error");
            });
    };

    $scope.submitSpecialUser = function() {
        var v = $(".newuserbox").val();
        $(".newuserbox").val("");
        console.log("Adding", v);

        $http.post('/api/bug/' + $routeParams.bugId + '/viewer/add/',
                   'email=' + encodeURIComponent(v),
                   {headers: {"Content-Type": "application/x-www-form-urlencoded"}}).
            success(function(data) {
                $scope.bug.also_visible_to = data.also_visible_to;
            }).
            error(function(data, code) {
                bAlert("Error " + code, "could not update visibility", "error");
            });
    };

    $scope.$on('Change', function(event, change) {
        if(change.bugid != $routeParams.bugId) {
            // not this bug
            return;
        }
        else if(change.user.md5 == $scope.auth.gravatar) {
            // wait this is me
            return;
        }
        else if(change.action == "commented on" && $scope.comments.length > 0 && change.time <= $scope.comments[$scope.comments.length-1].created_at) {
            // this is a comment on this bug, but we've already seen it
            return;
        } else if (change.time <= $scope.bug.modified_at) {
            // this is an update to this bug, but we've already seen it
            return;
        }
        // if we got this far, we should show the change
        alertAction = "changed";
        if(change.action == "commented on") {
            alertAction = "commented";
        }
        alertId = change.bugid + "-" + alertAction + "-by-" + change.user.md5;
        alert = { title: "Update", template: "change", change: change, context: $location.path(), id: alertId};
        cbuggGrowl.createAlert(alert);
    });
}

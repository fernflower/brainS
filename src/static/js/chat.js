var state = "chat";
var client = "";
var master = ""
var images = {};
var beeplong = new Audio("beep_long.mp3");
var beep = new Audio("beep.mp3");
var beep_int = null;

$(document).ready(function(){
    msg.focus();
});

// a callback for whoami
function whoami(data) {
    client = data.Name;
    master = data.MasterName;
    // hide control panel from all but master
    if (master != "" && client != master) {
        $("#master").hide()
    }
    return false;
}

// a callback for :listplayers command
function updatePlayers(data) {
    data = data.Text
    var players = $("#players");

    if (!isMaster()) {
        return false;
    }
    names = JSON.parse(data)
    var html = ""
    for (var k in names) {
        cls = (names[k]) ? "" : " progress-bar-warning";
        html += ("<span class='badge" + cls +"'>" + k + "</span>");
    }
    players.html(html);
    return false;
}

function isMaster() {
    return (client == master);
};

function getColor() {
    var range = "0123456789abcdef";
    var color = "";
    var i = 0;
    while(i < 6) {
        color += range[Math.floor(Math.random() * 16)];
        i++;
    }
    return color;
}

function changeState(newState) {
    var msg = $("#msg");
    var pressBut = $("#pressBut");

    clearInterval(beep_int);
    if (newState == "game") {
        pressBut.text("Answer");
        pressBut.focus();
    } else if (newState == "answer") {
        msg.attr("disabled", false);
    } else if (newState == "chat") {
        pressBut.text("Send");
        msg.attr("disabled", false);
        msg.focus();
    } else if (newState == "timeout") {
        beeplong.play();
    } else if (newState == "5sec") {
        beep.play();
        beep_int = setInterval(function(){beep.play();}, 1000);
    }
    state = newState;
    return false;
}

function appendLog(html) {
    var log = $("#log");
    log.append(html);
    return false;
}

function publishMessage(text, author) {
    var side = "left";
    var opp_side = "right";

    if (client == author) {
        side = "right";
        opp_side = "left";
    }
    if (!(author in images)) {
        img = "http://placehold.it/50/" + getColor() + "/fff&text=U";
        images[author] = img;
    }
    var html = "<li class='clearfix " + side + "'><span class='chat-img pull-" + opp_side + 
        "'><img src='" + images[author] + "' class='img-circle' /></span>" +
        "<div class='chat-body clearfix'><div class='header'><strong class='pull-" +
        opp_side + " primary-font'>" + author + "</strong></div><p>" + text + "</p></div></li>";
    return appendLog(html)
}

$(function() {

    var conn;
    var msg = $("#msg");
    var log = $("#log");
    var players = $("#players");
    var pressBut = $("#pressBut");

    pressBut.click(function() {
        if (!conn) {
            return false;
        }
        if (state == "chat") {
            if (!msg.val()) {
                return false;
            }
            conn.send(msg.val());
            msg.val("");
            return false
        }
        var toSend = !msg.val() ? "\n" : msg.val(); 
        conn.send(toSend);
        msg.val("");
        return false
    });

    $("#master_controls").on("show.bs.collapse", function(){
        conn.send(":master");
        $("#master").attr("disabled", true);
    });

    $(".master").click(function() {
        if (client != master) {
            return false;
        }
        // a hack, mind ids
        arr = this.id.split('_');
        cmd = (":" + arr[0]) + (arr.length > 1 ? " " + arr[1] : "");
        conn.send(cmd);
        return false;
    });

    if (window["WebSocket"]) {
        conn = new WebSocket("ws://localhost:9999/connect");
        conn.onclose = function(evt) {
            appendLog($("<div><b>Connection closed.</b></div>"))
        }
        conn.onmessage = function(evt) {
            data = JSON.parse(evt.data);
            if (data.Type == "control") {
                // treat Action as a callback to call
                window[data.Action](data);
                return false;
            }
            var text = data.Text
                publishMessage(data.Text, data.Name);
            // FIXME XXX damn, that's awful, change data from server to json (state, msg)
            if (text.search("Game Reset") > -1) {
                changeState("game");
            } else {
                changeState(data.State);
            }
        }
    } else {
        appendLog($("<div><b>Your browser does not support WebSockets.</b></div>"))
    }
});

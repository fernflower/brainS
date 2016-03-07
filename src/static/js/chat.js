var state = "chat";
var client = "";
var master = ""
var images = {};
var beeplong = new Audio("beep_long.mp3");
var beep = new Audio("beep.mp3");
var beep_int = null;

$(function() {

var conn;
var msg = $("#msg");
var log = $("#log");

function appendLog(html) {
    var d = log[0]
    var doScroll = d.scrollTop == d.scrollHeight - d.clientHeight;
    log.append(html)
    /*if (doScroll) {
        d.scrollTop = d.scrollHeight - d.clientHeight;
    }*/
}

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
    return appendLog(html, side)
}

function changeState(newState) {
    clearInterval(beep_int);
    if (newState == "game") {
        $("#pressBut").text("Answer");
        $("#pressBut").focus();
    } else if (newState == "answer") {
        msg.attr("disabled", false);
    } else if (newState == "chat") {
        $("#pressBut").text("Send");
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

$("#pressBut").click(function() {
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
        appendLog($("<div><b>Connection closed.</b></div>"), "left")
    }
    conn.onmessage = function(evt) {
        data = JSON.parse(evt.data);
        if (data.Type == "whoami") {
            client = data.Name;
            master = data.MasterName;
            if (master != "" && client != master) {
                $("#master").hide()
            }
            return
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
    appendLog($("<div><b>Your browser does not support WebSockets.</b></div>"), "left")
}
});
$(document).ready(function(){
    msg.focus();
})

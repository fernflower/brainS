var state = "chat";
var client = "";
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
    var html = "<li class='clearfix " + side + "'><span class='chat-img pull-'" + side + 
        "><img src='http://placehold.it/50/" + getColor() + "/fff&text=U' class='img-circle' /></span>" +
        "<div class='chat-body clearfix'><div class='header'><strong class='pull-" +
        opp_side + " primary-font'>" + author + "</strong></div><p>" + text + "</p></div></li>"
    return appendLog(html, side)
}

function changeState(newState) {
    if (newState == "game") {
        $("#pressBut").text("Answer");
        $("#pressBut").focus()
    } else if (newState == "answer") {
        msg.attr("disabled", false);
    } else if (newState == "chat") {
        $("#pressBut").text("Send");
        msg.attr("disabled", false);
        msg.focus()
    }
    state = newState
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
    if (state == "game") {
        // empty string is valid for game mode (button press)
        if (!msg.val()) {
            conn.send("\n");
            return false;
        }
        conn.send(msg.val());
        msg.val("");
        return false
    }
    return false
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
            isMaster = data.IsMaster;
            return
        }
        var text = data.Text
        publishMessage(data.Text, data.Name);
            // FIXME XXX damn, that's awful, change data from server to json (state, msg)
            if (data.State == "game") {
                changeState("game");
            } else if (data.State == "chat") {
                changeState("chat");
            } else if (text.search("your answer") > -1) {
                changeState("answer");
            } else if (text.search("Game Reset") > -1 || text.search("Time is Out") > -1) {
                changeState("game");
            }
    }
} else {
    appendLog($("<div><b>Your browser does not support WebSockets.</b></div>"), "left")
}
});
$(document).ready(function(){msg.focus()})

var state = "chat";
$(function() {

var conn;
var msg = $("#msg");
var log = $("#log");

function appendLog(msg, side) {
    var opp_side = "left";
    if (side == "left") {
        var opp_side = "right"
    }
    var d = log[0]
    var doScroll = d.scrollTop == d.scrollHeight - d.clientHeight;
    log.append("<li class='clearfix " + side + "'>" +"<div class='chat-body clearfix'><div class='header'><strong class='pull-" +
               opp_side + " primary-font'>Bhaumik Patel</strong></div><p>" + msg.text() + "</p></div></li>")
    /*if (doScroll) {
        d.scrollTop = d.scrollHeight - d.clientHeight;
    }*/
}

function changeState(newState) {
    if (newState == "game") {
        $("#pressBut").text("Answer");
        msg.attr("disabled", true);
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
        appendLog($("<div/>").text(evt.data), "left")
            // FIXME XXX damn, that's awful, change data from server to json (state, msg)
            if (evt.data.search("Game Mode") > -1) {
                changeState("game");
            } else if (evt.data.search("Chat Mode") > -1) {
                changeState("chat");
            } else if (evt.data.search("your answer") > -1) {
                changeState("answer");
            } else if (evt.data.search("Game Reset") > -1 || evt.data.search("Time is Out") > -1) {
                changeState("game");
            }
    }
} else {
    appendLog($("<div><b>Your browser does not support WebSockets.</b></div>"), "left")
}
});
$(document).ready(function(){msg.focus()})

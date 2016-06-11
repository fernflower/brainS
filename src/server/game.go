package server


import (
    "github.com/gorilla/websocket"
    "encoding/json"
    "fmt"
    "settings"
    "strconv"
    "strings"
    "time"
    "utils"
)


type Game struct {
    // array of joined players
    Players []*Player
    // chat, game, answer
    State string
    incoming chan string
    master *Player
    buttonPressed *Player
    // true if countdown has started
    time bool
    // notify when client wants to exit
    exit chan bool
    timer *time.Timer
    server *Server
}

func (game *Game) Master() string {
    if game.master != nil {
        return game.master.Name()
    }
    return ""
}

func (game *Game) GetPlayersOnline() []*Player {
    var online []*Player
    for _, cl := range game.Players {
        if !cl.disconnected {
            online = append(online, cl)
        }
    }
    return online
}

func (g *Game) formGameMessage(data string, msgType string, action string) Message {
    /* 2 mgsTypes are supported by UI:
       * 'plain' - any data with that type will be processed as simple text message;
       * 'control' - special type for maintanence messages (whoami, update etc). 
         Client will invoke function specified in action field with data passed as its
         argument.
    */
    msg := FormMessage(data)
    msg.Type = msgType
    msg.Action = action
    msg.State = g.State
    msg.MasterName = g.Master()
    return msg
}

func (game *Game) sendPlayersState() {
    // sends a Message with JSON data about all players in the form
    // {'name': can_answer}
    clients := make(map[string]bool)
    for _, cl := range game.GetPlayersOnline() {
        clients[cl.name] = cl.canAnswer
    }
    jsonstr, err := json.Marshal(clients)
    utils.ProcError(err)
    msg := game.formGameMessage(string(jsonstr), "control", "updatePlayers")
    if game.master != nil {
        game.master.outcoming <- msg.ToString()
    }
}

func (game *Game) UpdatePlayers() {
    for _, client := range game.Players {
        msg := game.formGameMessage("", "control", "whoami")
        // add client name for the client code to find out who is who
        msg.Name = client.Name()
        client.outcoming <- msg.ToString()
    }
}

func (game *Game) SystemMsg(data string, notify bool) {
    if notify {
        game.notifyListener(fmt.Sprintf("(system) %s", data))
    }
}

func (game *Game) Broadcast(msg string) {
    for _, client := range game.GetPlayersOnline() {
        client.outcoming <- msg
    }
    game.notifyListener(fmt.Sprintf("(broadcast) %s", msg))
}

func (game *Game) Inform(data string, client *Player) {
    // if data doesn't end in EOL, add one
    if !strings.HasSuffix(data, string(settings.EOL)) {
        data = data + string(settings.EOL)
    }
    client.outcoming <- data
    game.notifyListener(fmt.Sprintf("(whisper) %s", data))
}

// makes all clients be able to answer again
func (game *Game) Reset() {
    game.State = "game"
    game.time = false
    game.buttonPressed = nil
    if game.timer != nil {
        game.timer.Stop()
    }
    for _, client := range game.GetPlayersOnline() {
        client.canAnswer = true
    }
}

func (game *Game) SetMaster(client *Player) {
    game.master = client
    client.isMaster = true
}

// return an array of token strings
func sanitizeCommandString(cmd string) []string {
    cmd = strings.Replace(cmd, string(settings.EOL), "", 1)
    cmdSplit := strings.Split(cmd, " ")
    var cmdParts []string
    for i := 0; i < len(cmdSplit); i++ {
        if cmdSplit[i] != "" {
            cmdParts = append(cmdParts, cmdSplit[i])
        }
    }
    return cmdParts
}

func (game *Game) procTimeCmd(cmdParts []string, client *Player) {
    var seconds int
    var err error
    if len(cmdParts) > 1 {
        seconds, err = strconv.Atoi(cmdParts[1])
        if err != nil {
            game.Inform(fmt.Sprintf(
                "Argument of time should be an integer, not '%s'", cmdParts[1]), client)
                return
        }
    } else {
        seconds = settings.RoundTimeout
    }
    if game.State == "chat" {
        game.Inform("Enter game mode first!", client)
        return
    }
    if game.master != client {
        game.Inform("Only master can launch countdown!", client)
        return
    }
    game.buttonPressed = nil
    game.time = true
    // hope timer spawn is ok in terms of resources
    timeoutFunc :=  func() {
        if game.buttonPressed == nil {
            game.State = "timeout"
            game.Broadcast("===========Time is Out===========")
            for _, cl := range game.Players {
                cl.canAnswer = false
            }
            game.sendPlayersState()
        }
    }
    if seconds > 5 {
        game.timer = time.AfterFunc(
            time.Duration(seconds - 5) * time.Second, 
            func() {
                if game.buttonPressed == nil {
                    game.State = "5sec"
                    game.Broadcast("5 seconds left")
                }
                game.timer = time.AfterFunc(time.Duration(5) * time.Second, timeoutFunc)
            })
    } else {
        game.timer = time.AfterFunc(time.Duration(seconds) * time.Second, timeoutFunc)
    }
    game.Broadcast(fmt.Sprintf("===========%d seconds===========", seconds))
}

func (game *Game) ProcessCommand(cmd string, client *Player) {
    cmdParts := sanitizeCommandString(cmd)
    if cmdParts[0] == ":rename" && len(cmdParts) == 2 {
        newName := strings.Join(cmdParts[1:len(cmdParts)], " ")
        oldName := client.Name()
        client.name = newName
        game.UpdatePlayers()
        game.Broadcast(fmt.Sprintf("%s is now known as %s", oldName, newName))
    } else if cmdParts[0] == ":master" {
        if game.master != nil && client != game.master {
            // FIXME ping master first, make sure it exists
            game.SystemMsg(fmt.Sprintf("%s attempted to seize the crown!", client.Name()), false)
            game.Inform("The game has a master already", client)
            return
        }
        game.SetMaster(client)
        game.UpdatePlayers()
        game.Broadcast(fmt.Sprintf("%s is now the master of the game", client.Name()))
    }  else if cmdParts[0] == ":time" {
        game.procTimeCmd(cmdParts, client)
    } else if cmdParts[0] == ":reset" {
        if game.master != client {
            game.Inform("Only master can reset the game!", client)
            return
        }
        if game.State == "chat" {
            game.Inform("Enter game mode first!", client)
            return
        }
        game.Reset()
        game.Inform("======Game reset======", client)
    } else if cmdParts[0] == ":game" {
        if game.master != client {
            game.Inform("Only master can switch to game mode!", client)
            return
        }
        game.Reset()
        game.State = "game"
        game.Broadcast("===========Game Mode On===========")
    } else if cmdParts[0] == ":chat" {
        if game.master != client {
            game.Inform("Only master can switch to chat mode!", client)
            return
        }
        game.Reset()
        game.State = "chat"
        game.Broadcast("===========Chat Mode On===========")
    } else if cmdParts[0] == ":exit" {
        if game.master != client {
            game.Inform("Only master can shutdown server!", client)
            return
        }
        game.exit <- true
        game.Broadcast("Server will be shutdown!")
    } else {
        game.Inform(fmt.Sprintf(
            "Unknown command: '%s'", strings.Join(cmdParts, " ")), client)
    }
}

func (game *Game) procEventLoop(client *Player) {
    for {
        msgString := <- client.incoming
        // XXX yuck, error handling
        fmt.Println(msgString)
        data := FromString(msgString).Text
        if strings.HasPrefix(data, ":") {
            game.ProcessCommand(data, client)
            
        } else if data == "\n" {
            /* special case: in game mode ENTER press means button click
               a click prior :time command is considered as a false start
            */
            if game.State == "chat" {
                // do not send empty messages when chatting, that's not polite!
                continue
            }
            if !client.canAnswer || client != game.buttonPressed && game.buttonPressed != nil {
                game.Inform("You can't press button now", client)
                continue
            }
            if !game.time {
                game.Broadcast(fmt.Sprintf("%s has a false start!", client.Name()))
                client.canAnswer = false
                game.sendPlayersState()
                continue
            }
            game.buttonPressed = client
            game.State = "answer"
            game.Broadcast(fmt.Sprintf(
                "%s, your answer?", game.buttonPressed.Name()))
            } else if game.State == "answer" && client == game.buttonPressed && client.canAnswer {
                // answering a question in game mode
                client.canAnswer = false
                game.incoming <- msgString 
                game.State = "game"
            } else if game.State == "chat" {
                // chat mode
                game.incoming <- msgString
            } else {
                game.Inform("You can't chat right now!", client)
            }
            game.sendPlayersState()
        }
}

func (game *Game) Join(conn *websocket.Conn) *Player {
    clientNum := strconv.Itoa(len(game.Players) + 1)
    client := NewPlayer(
        conn, fmt.Sprintf("anonymous player %s", clientNum))
    // add client-game reference
    client.Game = game
    game.Players = append(game.Players, client)
    game.SystemMsg(
        fmt.Sprintf("'%s' has joined (%s). Total clients: %d",
                    client.name, client.conn.RemoteAddr(),
                    len(game.GetPlayersOnline())),
        true)
    // send client his registration data
    game.UpdatePlayers()
    msg := game.formGameMessage(
        fmt.Sprintf("'%s' has joined us!", client.Name()),
        "plain",
        game.State)
    game.Broadcast(msg.ToString())
    go game.procEventLoop(client)
    return client
}

func (game *Game) notifyListener(msg string) {
    // notify that client has been created
    ch := game.server.stateCh
    if ch != nil {
        ch <- msg
    }
}

func (game *Game) Listen() {
    go func() {
        for {
            select {
            case data := <-game.incoming:
                game.Broadcast(data)
            case <- game.exit:
                game.SystemMsg("Closing client connections..", false)
                for _, cl := range game.GetPlayersOnline() {
                    game.SystemMsg(fmt.Sprintf("Disconnecting client %s", cl.conn.RemoteAddr()), false)
                    cl.Exit()
                }
                // for bug-evading purposes only
                game.SystemMsg(fmt.Sprintf("Done! Players left: %d", len(game.GetPlayersOnline())), true)
                game.SystemMsg("Shutting down server..", false)
                game.server.listener.Stop()
                return
            }
        }
    }()
}

func NewGame() *Game {
    game := &Game{
        incoming: make(chan string),
        Players: make([]*Player, 0),
        exit: make(chan bool, 1),
        State: "chat",
    }
    game.Listen()

    return game
}

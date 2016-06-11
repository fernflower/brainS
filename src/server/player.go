package server


import (
    "github.com/gorilla/websocket"
    "fmt"
    "utils"
)


type Player struct {
    // a reference to game played
    Game *Game
    name string
    incoming chan string 
    outcoming chan string
    isMaster bool
    canAnswer bool
    conn *websocket.Conn
    // if true then already cleaned up
    disconnected bool
}

func (client *Player) Name() string {
    if client.isMaster {
        return "(master) " + client.name
    }
    return client.name
}

func (client *Player) SetName(name string) {
    client.name = name
}

func (client *Player) Read() {
    /*It is expected that all server->client communication is performed
    via Message objects.
    In client->server communication, to ease the pain of writing client UI
    and developing/parsing some custom format, a client sends a string of text
    that is converted to a Message according to the following rules:
    * if a string starts with a semicolon it is treated as a command
    * any other string is treated as a chat message/game answer
    */
    game := client.Game
    for {
        _, p, err := client.conn.ReadMessage()
        oe, ok := err.(*websocket.CloseError)
        if ok && (oe.Code == websocket.CloseNormalClosure || oe.Code == websocket.CloseGoingAway ||
        oe.Code == websocket.CloseNoStatusReceived) {
            game.SystemMsg(
                fmt.Sprintf("Player %s disconnected", client.conn.RemoteAddr()), true)
                client.Exit()
                return
        }
        if err != nil && client.disconnected {
            // XXX FIXME this read should not occur at all!!!
            game.SystemMsg("WARN: reading from a disconnected client", false)
            return
        }
        utils.ProcError(err)
        msg := client.formPlayerMessage(string(p))
        client.incoming <- msg.ToString()
    }
}

func (p *Player) Write() {
    for data := range p.outcoming {
        p.conn.WriteJSON(FormMessage(data))
    }
}

func (p *Player) formPlayerMessage(data string) Message {
    msg := FormMessage(data)
    msg.Name = p.Name()
    // FIXME is it really needed?
    msg.State = p.Game.State
    msg.MasterName = p.Game.Master()
    return msg
}

func (client *Player) Listen() {
    go client.Read()
    go client.Write()
}

func (client *Player) Exit() {
    defer func() {
        client.disconnected = true
        client.conn.Close()
    }()

    game := client.Game
    if game.master == client {
        game.master = nil
    }
}

func NewPlayer(conn *websocket.Conn, name string) *Player {
    client := &Player{name: name,
                     incoming: make(chan string),
                     outcoming: make(chan string),
                     canAnswer: true,
                     conn: conn}
    client.Listen()
    return client
}

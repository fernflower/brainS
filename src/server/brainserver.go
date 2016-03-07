package server


import (
    "github.com/gorilla/websocket"
    "encoding/json"
    "fmt"
    "listener"
    "net"
    "net/http"
    "settings"
    "strconv"
    "strings"
    "time"
    "utils"
)

type Client struct {
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

func (client *Client) GetName() string {
    if client.isMaster {
        return "(master) " + client.name
    }
    return client.name
}

func (client *Client) SetName(name string) {
    client.name = name
}

func (client *Client) Read() {
    game := client.Game
    for {
        _, p, err := client.conn.ReadMessage()
        oe, ok := err.(*websocket.CloseError)
        if ok && (oe.Code == websocket.CloseNormalClosure || oe.Code == websocket.CloseGoingAway ||
        oe.Code == websocket.CloseNoStatusReceived) {
            game.SystemMsg(
                fmt.Sprintf("Client %s disconnected", client.conn.RemoteAddr()), true)
                client.Exit()
                return
        }
        if err != nil && client.disconnected {
            // XXX FIXME this read should not occur at all!!!
            game.SystemMsg("WARN: reading from a disconnected client", false)
            return
        }
        utils.ProcError(err)
        m := Message{"plain", client.GetName(), string(p), game.GetMaster(), game.state}
        client.incoming <- m.ToString()
    }
}

func (client *Client) Write() {
    for data := range client.outcoming {
        var s map[string] interface{}
        if json.Unmarshal([]byte(data), &s) != nil {
            // XXX FIXME too bad to assume that in case of plain broadcast/inform master says that 
            msg := Message{"plain", "Brain Bot", data, client.Game.GetMaster(), client.Game.state}
            client.conn.WriteJSON(msg)
        } else {
            client.conn.WriteJSON(s)
        }
    }
}

func (client *Client) Listen() {
    go client.Read()
    go client.Write()
}

func (client *Client) Exit() {
    defer func() {
        client.disconnected = true
        client.conn.Close()
    }()

    game := client.Game
    if game.master == client {
        game.master = nil
    }
}

func NewClient(conn *websocket.Conn, name string) *Client {
    client := &Client{name: name,
                     incoming: make(chan string),
                     outcoming: make(chan string),
                     canAnswer: true,
                     conn: conn}
    client.Listen()
    return client
}

type Game struct {
    Clients []*Client
    incoming chan string
    master *Client
    buttonPressed *Client
    // chat, game, answer
    state string
    // true if countdown has started
    time bool
    // notify when client wants to exit
    exit chan bool
    timer *time.Timer
    server *Server
}

func (game *Game) GetMaster() string {
    if game.master != nil {
        return game.master.GetName()
    }
    return ""
}

func (game *Game) GetClientsOnline() []*Client {
    var online []*Client
    for _, cl := range game.Clients {
        if !cl.disconnected {
            online = append(online, cl)
        }
    }
    return online
}

func (game *Game) UpdateClients() {
    for _, client := range game.Clients {
        m := Message{"whoami", client.GetName(), "", game.GetMaster(), game.state}
        client.outcoming <- m.ToString()
    }
}

func (game *Game) SystemMsg(data string, notify bool) {
    if notify {
        game.notifyListener(fmt.Sprintf("(system) %s", data))
    }
}

func (game *Game) Broadcast(msg string) {
    for _, client := range game.GetClientsOnline() {
        client.outcoming <- msg
    }
    game.notifyListener(fmt.Sprintf("(broadcast) %s", msg))
}

func (game *Game) Inform(data string, client *Client) {
    // if data doesn't end in EOL, add one
    if !strings.HasSuffix(data, string(settings.EOL)) {
        data = data + string(settings.EOL)
    }
    client.outcoming <- data
    game.notifyListener(fmt.Sprintf("(whisper) %s", data))
}

// makes all clients be able to answer again
func (game *Game) Reset() {
    game.state = "game"
    game.time = false
    game.buttonPressed = nil
    if game.timer != nil {
        game.timer.Stop()
    }
    for _, client := range game.GetClientsOnline() {
        client.canAnswer = true
    }
}

func (game *Game) SetMaster(client *Client) {
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

func (game *Game) procTimeCmd(cmdParts []string, client *Client) {
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
    if game.state == "chat" {
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
            game.state = "timeout"
            game.Broadcast("===========Time is Out===========")
        }
    }
    if seconds > 5 {
        game.timer = time.AfterFunc(
            time.Duration(seconds - 5) * time.Second, 
            func() {
                if game.buttonPressed == nil {
                    game.state = "5sec"
                    game.Broadcast("5 seconds left")
                }
                game.timer = time.AfterFunc(time.Duration(5) * time.Second, timeoutFunc)
            })
    } else {
        game.timer = time.AfterFunc(time.Duration(seconds) * time.Second, timeoutFunc)
    }
    game.Broadcast(fmt.Sprintf("===========%d seconds===========", seconds))
}

func (game *Game) ProcessCommand(cmd string, client *Client) {
    cmdParts := sanitizeCommandString(cmd)
    if cmdParts[0] == ":rename" && len(cmdParts) == 2 {
        newName := strings.Join(cmdParts[1:len(cmdParts)], " ")
        oldName := client.GetName()
        client.name = newName
        game.Broadcast(fmt.Sprintf("%s is now known as %s", oldName, newName))
    } else if cmdParts[0] == ":master" {
        if game.master != nil && client != game.master {
            // FIXME ping master first, make sure it exists
            game.SystemMsg(fmt.Sprintf("%s attempted to seize the crown!", client.GetName()), false)
            game.Inform("The game has a master already", client)
            return
        }
        game.SetMaster(client)
        game.UpdateClients()
        game.Broadcast(fmt.Sprintf("%s is now the master of the game", client.GetName()))
    }  else if cmdParts[0] == ":time" {
        game.procTimeCmd(cmdParts, client)
    } else if cmdParts[0] == ":reset" {
        if game.master != client {
            game.Inform("Only master can reset the game!", client)
            return
        }
        if game.state == "chat" {
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
        game.state = "game"
        game.Broadcast("===========Game Mode On===========")
    } else if cmdParts[0] == ":chat" {
        if game.master != client {
            game.Inform("Only master can switch to chat mode!", client)
            return
        }
        game.Reset()
        game.state = "chat"
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

func (game *Game) procEventLoop(client *Client) {
    for {
        msgString := <- client.incoming
        // XXX yuck, error handling
        data := FromString(msgString).Text
        if strings.HasPrefix(data, ":") {
            game.ProcessCommand(data, client)
        } else if data == "\n" {
            /* special case: in game mode ENTER press means button click
               a click prior :time command is considered as a false start
            */
            if game.state == "chat" {
                // do not send empty messages when chatting, that's not polite!
                continue
            }
            if !client.canAnswer || client != game.buttonPressed && game.buttonPressed != nil {
                game.Inform("You can't press button now", client)
                continue
            }
            if !game.time {
                game.Broadcast(fmt.Sprintf("%s has a false start!", client.GetName()))
                client.canAnswer = false
                continue
            }
            game.buttonPressed = client
            game.state = "answer"
            game.Broadcast(fmt.Sprintf(
                "%s, your answer?", game.buttonPressed.GetName()))
            } else if game.state == "answer" && client == game.buttonPressed && client.canAnswer {
                // answering a question in game mode
                client.canAnswer = false
                game.incoming <- msgString 
                game.state = "game"
            } else if game.state == "chat" {
                // chat mode
                game.incoming <- msgString
            } else {
                game.Inform("You can't chat right now!", client)
            }
        }
}

func (game *Game) Join(conn *websocket.Conn) *Client {
    clientNum := strconv.Itoa(len(game.Clients) + 1)
    client := NewClient(
        conn, fmt.Sprintf("anonymous player %s", clientNum))
    // add client-game reference
    client.Game = game
    game.Clients = append(game.Clients, client)
    game.SystemMsg(
        fmt.Sprintf("'%s' has joined (%s). Total clients: %d",
                    client.name, client.conn.RemoteAddr(),
                    len(game.GetClientsOnline())),
        true)
    // send client his registration data
    game.UpdateClients()
    game.Broadcast(fmt.Sprintf("'%s' has joined us!", client.GetName()))
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
                for _, cl := range game.GetClientsOnline() {
                    game.SystemMsg(fmt.Sprintf("Disconnecting client %s", cl.conn.RemoteAddr()), false)
                    cl.Exit()
                }
                // for bug-evading purposes only
                game.SystemMsg(fmt.Sprintf("Done! Clients left: %d", len(game.GetClientsOnline())), true)
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
        Clients: make([]*Client, 0),
        exit: make(chan bool, 1),
        state: "chat",
    }
    game.Listen()

    return game
}

type Server struct {
    *http.Server
    Games []*Game
    joins chan websocket.Conn
    listener *listener.StoppableListener
    // a channel passed from outside to monitor up/down state
    stateCh chan string
}

func (server *Server) addGame() *Game{
    game := NewGame()
    game.server = server
    server.Games = append(server.Games, game)
    return game
}

func (server *Server) notifyListener(msg string) {
    if server.stateCh != nil {
        server.stateCh <- msg
    }
}

func NewServer(host string, port int, stateCh chan string) (*Server) {
    ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
    utils.ProcError(err)
    // use stoppable listener further on
    sl, err := listener.New(ln)
    utils.ProcError(err)
    httpServer := &http.Server{Addr: fmt.Sprintf("%s:%d", host, port)}
    s := &Server{httpServer, make([]*Game, 0), make(chan websocket.Conn), sl, stateCh}
    return s
}

func (s *Server) Start(){
    var upgrader = websocket.Upgrader{}
    game := s.addGame()
    game.SystemMsg("Launching Brain Server...", true)
    handleConn := func(w http.ResponseWriter, r *http.Request) {
        conn, err := upgrader.Upgrade(w, r, nil)
        utils.ProcError(err)
        s.joins <- *conn
    }
    go func() {
        for {
            select {
                case conn := <-s.joins:
                    game.Join(&conn)
                default:
            }
        }
    }()
    http.Handle("/", http.FileServer(http.Dir("./static")))
    http.HandleFunc("/connect", handleConn)
    s.Serve(s.listener)
    defer s.Stop()
}

func (s *Server) Stop() {
    for _, game := range s.Games {
        game.exit <- true
    }
}

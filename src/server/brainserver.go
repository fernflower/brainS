package server


import (
    "bufio"
    "io"
    "fmt"
    "listener"
    "net"
    "settings"
    "strconv"
    "strings"
    "sync"
    "time"
    "utils"
)

type Client struct {
    // a reference to game played
    Game *Game
    name string
    incoming chan string
    outcoming chan string
    reader *bufio.Reader
    writer *bufio.Writer
    isMaster bool
    canAnswer bool
    // FIXME probably will needed to determine button click
    // precedence regardless of race conditions
    pressTime time.Time
    // XXX FIXME Do we need to close it manually?
    conn net.Conn
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
    for {
        line, err := client.reader.ReadString(settings.EOL)
        if err == io.EOF {
            fmt.Println("Client disconnected")
            client.Exit()
            return
        } else if err != nil && client.disconnected {
            // XXX FIXME this read should not occur at all!!!
            fmt.Println("WARN: reading from a disconnected client")
            return
        }
        utils.ProcError(err)
        client.incoming <- line
    }
}

func (client *Client) Write() {
    for data := range client.outcoming {
        client.writer.WriteString(data)
        client.writer.Flush()
    }
}

func (client *Client) Listen() {
    go client.Read()
    go client.Write()
}

func (client *Client) Exit() {
    // XXX FIXME figure out the proper way to close a connection
    defer func() {
        client.disconnected = true
        client.conn.Close()
    }()

    game := client.Game
    if game.master == client {
        game.master = nil
    }
}

func NewClient(conn net.Conn, name string) *Client {
    reader := bufio.NewReader(conn)
    writer := bufio.NewWriter(conn)
    client := &Client{name: name,
                     reader: reader,
                     writer: writer,
                     incoming: make(chan string),
                     outcoming: make(chan string),
                     canAnswer: true, 
                     conn: conn}
    client.Listen()
    return client
}

type Game struct {
    Clients []*Client
    joins chan net.Conn
    incoming chan string
    timeout chan time.Time
    master *Client
    buttonPressed *Client
    // when true any button click prior to time=true
    // means false start
    gameMode bool
    // true if countdown has started
    time bool
    // notify when client wants to exit
    exit chan bool
    server *Server
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

func (game *Game) Broadcast(data string) {
    // if data doesn't end in EOL, add one
    if !strings.HasSuffix(data, string(settings.EOL)) {
        data = data + string(settings.EOL)
    }
    for _, client := range game.Clients {
        client.outcoming <- data
    }
    game.notifyListener(fmt.Sprintf("(broadcast) %s", data))
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
    game.gameMode = true
    game.time = false
    game.buttonPressed = nil
    for _, client := range game.Clients {
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
    if !game.gameMode {
        game.Inform("Enter game mode first!", client)
        return
    }
    if game.master != client {
        game.Inform("Only master can launch countdown!", client)
        return
    }
    game.buttonPressed = nil
    game.time = true
    go func() {
        game.timeout <- <- time.After(
            time.Duration(seconds) * time.Second)
        }()
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
            fmt.Println(fmt.Sprintf("%s attempted to seize the crown!", client.GetName()))
            game.Inform("The game has a master already", client)
            return
        }
        game.SetMaster(client)
        game.Broadcast(fmt.Sprintf("%s is now the master of the game", client.GetName()))
    }  else if cmdParts[0] == ":time" {
        game.procTimeCmd(cmdParts, client)
    } else if cmdParts[0] == ":reset" {
        if game.master != client {
            game.Inform("Only master can reset the game!", client)
            return
        }
        if !game.gameMode {
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
        game.gameMode = true
        game.Broadcast("===========Game Mode On===========")
    } else if cmdParts[0] == ":chat" {
        if game.master != client {
            game.Inform("Only master can switch to chat mode!", client)
            return
        }
        game.Reset()
        game.gameMode = false
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
        data := <- client.incoming
        if strings.HasPrefix(data, ":") {
            game.ProcessCommand(data, client)
        } else if data == "\n" {
            /* special case: in game mode ENTER press means button click
               
               a click prior :time command is considered as a false start 
            */
            if !game.gameMode {
                // do not send empty messages when chatting, that's not polite!
                continue
            }
            if !client.canAnswer {
                game.Inform("You can't press button now", client)
                continue
            }
            if !game.time {
                game.Broadcast(fmt.Sprintf("%s has a false start!", client.GetName()))
                client.canAnswer = false
                continue
            }
            game.buttonPressed = client
            game.Broadcast(fmt.Sprintf(
                "%s, your answer?", game.buttonPressed.GetName()))
            } else if game.gameMode && client == game.buttonPressed && client.canAnswer {
                // answering a question in game mode
                client.canAnswer = false
                toSend := fmt.Sprintf("[%s] %s", client.GetName(), data)
                game.incoming <- toSend
            } else if !game.gameMode {
                // chat mode
                toSend := fmt.Sprintf("[%s] %s", client.GetName(), data)
                game.incoming <- toSend
            } else {
                game.Inform("You can't chat right now!", client)
            }
        }
}

func (game *Game) Join(conn net.Conn) *Client {
    clientNum := strconv.Itoa(len(game.Clients) + 1)
    client := NewClient(
        conn, fmt.Sprintf("anonymous player %s", clientNum))
    // add client-game reference
    client.Game = game
    game.Clients = append(game.Clients, client)
    fmt.Println(fmt.Sprintf(
        "'%s' has joined (%s). Total clients: %d", client.name, client.conn.RemoteAddr(), len(game.Clients)))
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
            case conn := <-game.joins:
                game.Join(conn)
            case <- game.timeout:
                if game.buttonPressed == nil {
                    game.Broadcast("===========Time is Out===========")
                    game.Reset()
                }
            case <- game.exit:
                fmt.Println("Closing client connections..")
                for _, cl := range game.Clients {
                    fmt.Println(fmt.Sprintf("Disconnecting client %s", cl.conn.RemoteAddr()))
                    if !cl.disconnected {
                        cl.Exit()
                    }
                }
                // for bug-evading purposes only
                fmt.Println(fmt.Sprintf("Done! Clients left: %d", len(game.GetClientsOnline())))
                fmt.Println("Shutting down server..")
                game.server.listener.Stop()
                return
            }
        }
    }()
}

func NewGame() *Game {
    game := &Game{
        incoming: make(chan string),
        timeout: make(chan time.Time),
        Clients: make([]*Client, 0),
        joins: make(chan net.Conn),
        exit: make(chan bool, 1),
    }
    game.Listen()

    return game
}

type Server struct {
    Games []*Game
    // a channel passed from outside to monitor up/down state
    listener *listener.StoppableListener
    stateCh chan string
    wg *sync.WaitGroup
}

func (server *Server) addGame() *Game{
    // for server start sync
    game := NewGame()
    game.server = server
    server.Games = append(server.Games, game)
    return game
}

func (server *Server) notifyListener(msg string) {
    // notify that client has been created
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
    s := &Server{make([]*Game, 0), sl, stateCh, &sync.WaitGroup{}}
    return s
}

func (s *Server) Start(){
    fmt.Println("Launching Brain Server...")
    s.notifyListener("server started")
    game := s.addGame()
    for {
        conn, err := s.listener.Accept()
        if err == listener.StoppedError {
            s.notifyListener("server shutdown")
            return
        } else {
            utils.ProcError(err)
        }
        game.joins <- conn
    }
    s.Stop()
}

func (s *Server) Stop() {
    for _, game := range s.Games {
        game.exit <- true
    }
}

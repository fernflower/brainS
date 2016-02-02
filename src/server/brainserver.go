package server


import ("bufio"
        "fmt"
        "net"
        "settings"
        "strconv"
        "strings"
        "time"
        "utils"
    )

type Client struct {
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
}

func (client *Client) GetName() string {
    if client.isMaster {
        return "(master) " + client.name
    }
    return client.name
}

func (client *Client) Read() {
    for {
        line, err := client.reader.ReadString(settings.EOL)
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

func NewClient(conn net.Conn, name string) *Client {
    reader := bufio.NewReader(conn)
    writer := bufio.NewWriter(conn)
    client := &Client{name: name,
                     reader: reader,
                     writer: writer,
                     incoming: make(chan string),
                     outcoming: make(chan string),
                     canAnswer: true}
    client.Listen()
    return client
}

type Game struct {
    clients []*Client
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
}

type Message struct {
    // fields should be exportable to ease the pain of testing
    Text string
    Visibility string
}

func (game *Game) Broadcast(data string) {
    // if data doesn't end in EOL, add one
    if !strings.HasSuffix(data, string(settings.EOL)) {
        data = data + string(settings.EOL)
    }
    for _, client := range game.clients {
        client.outcoming <- data
    }
}

func (game *Game) Inform(data string, client *Client) {
    // if data doesn't end in EOL, add one
    if !strings.HasSuffix(data, string(settings.EOL)) {
        data = data + string(settings.EOL)
    }
    client.outcoming <- data
}

// makes all clients be able to answer again
func (game *Game) Reset() {
    game.gameMode = true
    game.time = false
    game.buttonPressed = nil
    for _, client := range game.clients {
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

func (game *Game) procTimeCmd(cmdParts []string, client *Client) *Message {
    var seconds int
    var err error
    if len(cmdParts) > 1 {
        seconds, err = strconv.Atoi(cmdParts[1])
        if err != nil {
            return &Message{fmt.Sprintf(
                "Argument of time should be an integer, not '%s'", cmdParts[1]), "client"}
            }
        } else {
            seconds = settings.RoundTimeout
        }
    if !game.gameMode {
        return &Message{"Enter game mode first!", "client"}
    }
    if game.master != client {
        return &Message{"Only master can launch countdown!", "client"}
    }
    game.buttonPressed = nil
    game.time = true
    go func() {
        game.timeout <- <- time.After(
            time.Duration(seconds) * time.Second)
        }()
    return &Message{fmt.Sprintf("===========%d seconds===========", seconds), "all"}
}

func (game *Game) modeSwitch(mode string, client *Client) (*Message, bool) {
    if game.master != client {
        return &Message{fmt.Sprintf("Only master can switch to %s mode!", mode), "client"}, false
    }
    game.Reset()
    return &Message{fmt.Sprintf("===========%s Mode On===========", mode), "all"}, true
}

func (game *Game) ProcessCommand(cmd string, client *Client) *Message {
    cmdParts := sanitizeCommandString(cmd)
    if cmdParts[0] == ":register" && len(cmdParts) == 2 {
        newName := strings.Join(cmdParts[1:len(cmdParts)], " ")
        oldName := client.GetName()
        client.name = newName
        return &Message{fmt.Sprintf("%s is now known as %s", oldName, newName), "all"}
    } else if cmdParts[0] == ":master" {
        if game.master != nil && client != game.master {
            // FIXME ping master first, make sure it exists
            fmt.Println(fmt.Sprintf("%s attempted to seize the crown!", client.GetName()))
            return &Message{"The game has a master already", "client"}
        }
        game.SetMaster(client)
        return &Message{fmt.Sprintf("%s is now the master of the game", client.GetName()), "all"}
    } else if cmdParts[0] == ":game" {
        if game.master != client {
            return &Message{"Only master can switch to game mode!", "client"}
        }
        game.gameMode = true
        return &Message{"===========Game Mode On===========", "all"}
    } else if cmdParts[0] == ":time" {
        return game.procTimeCmd(cmdParts, client)
    } else if cmdParts[0] == ":reset" {
        if game.master != client {
            return &Message{"Only master can reset game!", "client"}
        }
        game.Reset()
        return &Message{"======Game reset======", "client"}
    } else if cmdParts[0] == ":game" || cmdParts[0] == ":chat" {
        msg, done := game.modeSwitch(cmdParts[0], client)
        if !done {
            return msg
        }
        if cmdParts[0] == ":game" {
            game.gameMode = true
        } else {
            game.gameMode = false
        }
        return msg
    } else {
        return &Message{fmt.Sprintf(
            "Unknown command: '%s'", strings.Join(cmdParts, " ")), "client"}
    }
}

func (game *Game) procEventLoop(client *Client) {
    for {
        data := <- client.incoming
        if strings.HasPrefix(data, ":") {
            func(message *Message){
                if message.Visibility == "all" {
                    game.Broadcast(message.Text)
                } else {
                    game.Inform(message.Text, client)
                }
            }(game.ProcessCommand(data, client))
        } else if data == "\n" {
            /* special case: in game mode ENTER press means button click
               
               a click prior :time command is considered as a false start 
            */
            if !game.gameMode {
                // do not send empty messages when chatting, that's not polite!
                continue
            }
            if !client.canAnswer {
                game.Inform("You can't press button this round", client)
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
                game.Inform("You cannot chat right now!", client)
            }
        }
}

func (game *Game) Join(conn net.Conn) {
    clientNum := strconv.Itoa(len(game.clients) + 1)
    client := NewClient(
        conn, fmt.Sprintf("anonymous player %s", clientNum))
    game.clients = append(game.clients, client)
    game.Broadcast(fmt.Sprintf("'%s' has joined us!", client.GetName()))
    go game.procEventLoop(client)
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
            }
        }
    }()
}

func NewGame() *Game {
    game := &Game{
        incoming: make(chan string),
        timeout: make(chan time.Time),
        clients: make([]*Client, 0),
        joins: make(chan net.Conn),
    }
    game.Listen()

    return game
}

func StartServer(host string, port int) {
    game := NewGame()
    fmt.Println("Launching Brain Server...")
    ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
    utils.ProcError(err)
    for {
        conn, err := ln.Accept()
        utils.ProcError(err)
        game.joins <- conn
    }
}

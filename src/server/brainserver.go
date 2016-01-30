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
    id string
    name string
    incoming chan string
    outcoming chan string
    reader *bufio.Reader
    writer *bufio.Writer
    isMaster bool
    falseStart bool
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

func NewClient(conn net.Conn, name string, id string) *Client {
    reader := bufio.NewReader(conn)
    writer := bufio.NewWriter(conn)
    client := &Client{name: name,
                     reader: reader,
                     writer: writer,
                     incoming: make(chan string),
                     outcoming: make(chan string),
                     id: id}
    client.Listen()
    return client
}

type Game struct {
    clients []*Client
    joins chan net.Conn
    incoming chan string
    timeout chan time.Time
    master *Client
    active bool
    time bool
    buttonPressed *Client
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

func (game *Game) Reset() {
    game.time = false
    game.buttonPressed = nil
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

func (game *Game) ProcessCommand(cmd string, client *Client) {
    cmdParts := sanitizeCommandString(cmd)
    if cmdParts[0] == ":register" && len(cmdParts) == 2 {
        newName := strings.Join(cmdParts[1:len(cmdParts)], " ")
        game.Broadcast(fmt.Sprintf("%s is now known as %s", client.GetName(), newName))
        client.name = newName
    } else if cmdParts[0] == ":master" {
        if game.master != nil && client != game.master {
            // FIXME ping master first, make sure it exists
            fmt.Println(fmt.Sprintf("%s attempted to seize the crown!", client.GetName()))
            game.Inform("The game has a master already", client)
            return
        }
        game.Broadcast(fmt.Sprintf("%s is now the master of the game", client.GetName()))
        client.isMaster = true
        game.master = client
    } else if cmdParts[0] == ":start" {
        if game.master != client {
            game.Inform("Only master can start a new game!", client)
            return
        }
        game.active = true
        game.Broadcast("===========Next Round===========")
    } else if cmdParts[0] == ":time" {
        if game.master != client {
            game.Inform("Only master can launch countdown!", client)
            return
        }
        if !game.active {
            game.Inform("Game must be started first!", client)
            return
        }
        game.Reset()
        game.time = true
        go func() {
            game.timeout <- <- time.After(
                time.Duration(settings.RoundTimeout) * time.Second)
        }()
    } else {
        fmt.Println(fmt.Sprintf("Unknown command: '%s'", cmd))
    }
}

func (game *Game) Join(conn net.Conn) {
    clientId := strconv.Itoa(len(game.clients) + 1)
    client := NewClient(
        conn, fmt.Sprintf("anonymous player %s", clientId), clientId)
    game.clients = append(game.clients, client)
    game.Broadcast(fmt.Sprintf("'%s' has joined us!\n", client.GetName()))
    go func() {
        for {
            data := <- client.incoming
            if strings.HasPrefix(data, ":") {
                game.ProcessCommand(data, client)
            } else if data == "\n" {
                /* special case: in game mode, after :time command,
                ENTER press means button click
                */
                game.buttonPressed = client
                game.Broadcast(fmt.Sprintf(
                    "%s, your answer?", game.buttonPressed.GetName()))
                } else {
                    toSend := fmt.Sprintf("[%s] %s", client.GetName(), data)
                    game.incoming <- toSend
                }
            }
        }()
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

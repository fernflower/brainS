package server


import ("bufio"
        "fmt"
        "net"
        "settings"
        "strconv"
        "strings"
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
    master *Client
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

func (game *Game) ProcessCommand(cmd string, client *Client) {
    cmd = strings.Replace(cmd, string(settings.EOL), "", 1)
    cmdSplit := strings.Split(cmd, " ")
    var cmdParts []string
    for i := 0; i < len(cmdSplit); i++ {
        if cmdSplit[i] != "" {
            cmdParts = append(cmdParts, cmdSplit[i])
        }
    }
    if cmdParts[0] == ":register" && len(cmdParts) == 2 {
        newName := strings.Join(cmdParts[1:len(cmdParts)], " ")
        game.Broadcast(fmt.Sprintf("%s is now known as %s", client.GetName(), newName))
        client.name = newName
    } else if cmdParts[0] == ":master" {
        if game.master != nil && client != game.master {
            // FIXME ping master first, make sure it exists
            fmt.Println(fmt.Sprintf("%s attempted to seize the crown!", client.GetName()))
            return
        }
        game.Broadcast(fmt.Sprintf("%s is now the master of the game", client.GetName()))
        client.isMaster = true
        game.master = client
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
            }
        }
    }()
}

func NewGame() *Game {
    game := &Game{
        incoming: make(chan string),
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

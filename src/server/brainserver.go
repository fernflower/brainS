package server


import ("bufio"
        "net"
        "fmt"
        "settings"
        "strconv"
        "utils")

type Client struct {
    id string
    name string
    incoming chan string
    outcoming chan string
    reader *bufio.Reader
    writer *bufio.Writer
    isMaster bool
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
}

func (game *Game) Broadcast(data string) {
    for _, client := range game.clients {
        client.outcoming <- data
    }
}

func (game *Game) Join(conn net.Conn) {
    clientId := strconv.Itoa(len(game.clients) + 1)
    client := NewClient(
        conn, fmt.Sprintf("anonymous player %s", clientId), clientId)
    game.clients = append(game.clients, client)
    game.Broadcast(fmt.Sprintf("'%s' has joined us!\n", client.name))
    go func() {
        for {
            toSend := fmt.Sprintf("[%s] ", client.name) + <- client.incoming
            game.incoming <- toSend
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

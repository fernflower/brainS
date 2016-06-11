package server


import (
    "github.com/gorilla/websocket"
    "fmt"
    "listener"
    "net"
    "net/http"
    "utils"
)


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
    return &Server{httpServer, make([]*Game, 0), make(chan websocket.Conn), sl, stateCh}
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

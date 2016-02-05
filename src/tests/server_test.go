package tests

import (
    "fmt"
    "net"
    "server"
    "testing"
    "strings"
    "utils"
)

func createClient(s *server.Server, stateCh chan string, name string, master bool) *server.Client {
    _, err := net.Dial("tcp", "127.0.0.1:9999")
    utils.ProcError(err)
    // FIXME XXX uncapsulation issues
    game := s.Games[0]
    for {
        select {
        case <- stateCh:
            cl := game.Clients[len(game.Clients) - 1]
            cl.SetName(name)
            if master {
                game.SetMaster(cl)
            }
            return cl
        default:
        }
    }
}

func startServer(stateCh chan string) *server.Server{
    s := server.NewServer("127.0.0.1", 9999, stateCh)
    go s.Start()
    for {
        select {
        case <- stateCh:
            fmt.Println("Server started successfully")
            return s
        default:
        }
    }
}

func stopServer(s *server.Server, stateCh chan string) {
    go func() {
        s.Stop()
    }()
    for {
        select {
        case <- stateCh:
            fmt.Println("Server shutdown successfully")
            return
        default:
        }
    }
}

func testGameCommands(t *testing.T, s *server.Server, stateCh chan string) {
    if len(s.Games) != 1 {
        t.Errorf("At least one game should be started!")
    }
    // create clients
    masterClient := createClient(s, stateCh, "bb-8", true)
    client := createClient(s, stateCh, "c-3po", false)
    // XXX incapsulation issues
    game := client.Game
    if !strings.HasPrefix(masterClient.GetName(), "(master)") {
        t.Errorf("Name should have a reference to master, not '%s'", masterClient.GetName())
    }
    // make sure that non-master can't use game commands
    commands := map[string]string {
        ":game": "Only master can switch to game mode!",
        ":reset": "Only master can reset the game!",
        ":time": "Enter game mode first!",
        ":master": "The game has a master already"}
    for cmd, expected := range commands {
        msg := game.ProcessCommand(cmd, client)
        if msg.Text != expected {
            t.Errorf("Expected '%s', not '%s'", expected, msg.Text)
        }
    }
    // master can use any of the following
    masterCommands := map[string]string {
        ":game": "===========Game Mode On===========",
        ":reset": "======Game reset======",
        ":time 15": "===========15 seconds===========",
        ":master": "(master) bb-8 is now the master of the game"}
    for cmd, expected := range masterCommands {
        msg := game.ProcessCommand(cmd, masterClient)
        if msg.Text != expected {
            t.Errorf("Expected '%s', not '%s'", expected, msg.Text)
        }
    }
}

func testChatCommands(t *testing.T, s *server.Server, stateCh chan string) {
    if len(s.Games) != 1 {
        t.Errorf("At least one game should be started!")
    }
    // create clients
    masterClient := createClient(s, stateCh, "m1", true)
    client := createClient(s, stateCh, "cl1", false)
    // XXX incapsulation issues
    game := client.Game
    commands := map[string]string {
        ":rename Arthur_Dent": fmt.Sprintf("%s is now known as %s", client.GetName(), "Arthur_Dent"),
        ":chat": "===========Chat Mode On==========="}
    for cmd, expected := range commands {
        msg := game.ProcessCommand(cmd, masterClient)
        if msg.Text != expected {
            t.Errorf("Expected '%s', not '%s'", expected, msg.Text)
        }
    }
}

func TestStartStop(t *testing.T) {
    stateCh := make(chan string)
    s := startServer(stateCh)
    client1 := createClient(s, stateCh, "client1", false)
    client2 := createClient(s, stateCh, "client2", false)
    client1.Exit()
    client2.Exit()
    stopServer(s, stateCh)
}

func TestMain(t *testing.T) {
    stateCh := make(chan string)
    funcs := []func(*testing.T, *server.Server, chan string){testGameCommands, testChatCommands}
    for _, f := range funcs {
        s := startServer(stateCh)
        f(t, s, stateCh)
        stopServer(s, stateCh)
    }
}

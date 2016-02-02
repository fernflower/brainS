package tests

import (
    "net"
    "server"
    "testing"
    "strings"
    "time"
    "utils"
)

func createClient() *server.Client {
    conn, err := net.Dial("tcp", "127.0.0.1:9999")
    utils.ProcError(err)
    clientMaster := server.NewClient(conn, "mc")
    return clientMaster
}

func startServer() {
    go server.StartServer("127.0.0.1", 9999)
    // FIXME not the best way to make sure server is up
    time.Sleep(5)
}

func TestGameCommands(t *testing.T) {
    startServer()
    game := server.NewGame()
    // create clients
    masterClient := createClient()
    game.SetMaster(masterClient)
    ordinaryClient := createClient()
    if !strings.HasPrefix(masterClient.GetName(), "(master)") {
        t.Errorf("Name should have a reference to master, not '%s'", masterClient.GetName())
    }
    msg := game.ProcessCommand(":master", ordinaryClient)
    if msg.Text != "The game has a master already" {
        t.Errorf("Answer should be 'The game has a master already', not '%s'", msg.Text)
    }
}

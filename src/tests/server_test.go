package tests

import (
    "fmt"
    "net"
    "server"
    "testing"
    "settings"
    "strings"
    "utils"
)

var stateCh chan string = make(chan string)

func assert(expected string, actual string, t *testing.T) {
    if actual != expected {
        t.Errorf("Expected '%s', not '%s'", expected, actual)
    }
}

func waitForData() string {
    for {
        select {
        case data:= <- stateCh:
            if strings.HasSuffix(data, string(settings.EOL)) {
                data = strings.Replace(data, string(settings.EOL), "", 1)
            }
            return fmt.Sprintf("%s", data)
        default:
        }
    }
}

func connect() (net.Conn, string) {
    conn, err := net.Dial("tcp", "127.0.0.1:9999")
    utils.ProcError(err)
    return conn, waitForData()
}

// same as connect(), but sets up proper client name and master
func enter(name string, master bool, t *testing.T) net.Conn {
    conn, data := connect()
    if !strings.HasSuffix(data, "has joined us!") {
        t.Errorf("Not the thing expected: '%s'", data)
    }
    // FIXME check return value?
    actual := getResponse(conn, fmt.Sprintf(":rename %s", name))
    if !strings.HasSuffix(actual, fmt.Sprintf("is now known as %s", name)) {
        t.Errorf("Not the thing expected: '%s'", actual)
    }
    if master {
        actual = getResponse(conn, ":master")
        if !strings.HasSuffix(actual, "is now the master of the game") {
            t.Errorf("Not the thing expected: '%s'", actual)
        }
    }
    return conn
}

func getResponse(conn net.Conn, data string) string {
    if !strings.HasSuffix(data, string(settings.EOL)) {
        data = data + string(settings.EOL)
    }
    fmt.Fprintf(conn, data)
    return waitForData()
}

func startServer() (*server.Server, string) {
    s := server.NewServer("127.0.0.1", 9999, stateCh)
    go s.Start()
    return s, waitForData()
}

func stopServer(s *server.Server) string {
    go s.Stop()
    return waitForData()
}

func TestChatCommands(t *testing.T) {
    s, _ := startServer()
    // create 2 ordinary clients
    // would-be master
    connM, actualM := connect()
    expected := "(broadcast) 'anonymous player 1' has joined us!"
    assert(expected, actualM, t)
    // ordinary client
    conn, actual := connect()
    expected = "(broadcast) 'anonymous player 2' has joined us!"
    assert(expected, actual, t)
    // :master - create a master
    masterActual := getResponse(connM, ":master")
    expectedMaster := "(broadcast) (master) anonymous player 1 is now the master of the game"
    assert(expectedMaster, masterActual, t)
    // :master - make sure no 2 masters can exist
    clientActual := getResponse(conn, ":master")
    expected = "(whisper) The game has a master already"
    assert(expected, clientActual, t)
    // :rename
    expectedMaster = "(broadcast) anonymous player 2 is now known as ArthurDent"
    actual = getResponse(conn, ":rename ArthurDent")
    assert(expectedMaster, actual, t)
    expected = "(broadcast) (master) anonymous player 1 is now known as FordPrefect"
    actual = getResponse(connM, ":rename FordPrefect")
    assert(expected, actual, t)
    // :chat
    msg := "All right. How would you react if I said that I'm" +
    " not from Guildford at all, but from a smal planet somewhere in" +
    "the vicinity of Betelgeuse?"
    expected = "(broadcast) [(master) FordPrefect] " + msg
    actual = getResponse(connM, msg)
    assert(expected, actual, t)
    msg = "I don't know. Why, do you think it's the sort of" +
    " thing you're likely to say?"
    expected = "(broadcast) [ArthurDent] " + msg
    actual = getResponse(conn, msg)
    assert(expected, actual, t)
    stopServer(s)
}

func TestGameCommands(t *testing.T) {
    s, _ := startServer()
    // create 2 clients
    connM := enter("Team1", true, t)
    conn := enter("Team2", false, t)
    // make sure that non-master can't use game commands
    commands := map[string]string {
        ":game": "(whisper) Only master can switch to game mode!",
        ":reset": "(whisper) Only master can reset the game!",
        ":time": "(whisper) Enter game mode first!"}
    for cmd, expected := range commands {
        actual := getResponse(conn, cmd)
        assert(expected, actual, t)
    }
    // game commands are ok for master
    commands = map[string]string {
        ":game": "(broadcast) ===========Game Mode On===========",
        ":reset": "(whisper) ======Game reset======",
        ":time 15": "(broadcast) ===========15 seconds==========="}
    for cmd, expected := range commands {
        actual := getResponse(connM, cmd)
        assert(expected, actual, t)
    }
    stopServer(s)
}

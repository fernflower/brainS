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

func disconnect(conn net.Conn) {
    conn.Close()
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
    // create master and 2 clients
    connM := enter("Master", true, t)
    conn1 := enter("Team2", false, t)
    enter("Team1", false, t)
    // make sure that non-master can't use game commands
    commands := map[string]string {
        ":game": "(whisper) Only master can switch to game mode!",
        ":reset": "(whisper) Only master can reset the game!",
        ":time": "(whisper) Enter game mode first!"}
    for cmd, expected := range commands {
        actual := getResponse(conn1, cmd)
        assert(expected, actual, t)
    }
    // game commands are ok for master
    // enter game mode
    assert("(broadcast) ===========Game Mode On===========",
           getResponse(connM, ":game"), t)
    commands = map[string]string {
        ":reset": "(whisper) ======Game reset======",
        ":time 15": "(broadcast) ===========15 seconds==========="}
    for cmd, expected := range commands {
        actual := getResponse(connM, cmd)
        assert(expected, actual, t)
    }
    stopServer(s)
}

func TestMultiplePress(t *testing.T) {
    s, _ := startServer()
    // create master and 2 clients
    connM := enter("Master", true, t)
    conn1 := enter("Team2", false, t)
    conn2 := enter("Team1", false, t)
    // enter game mode
    assert("(broadcast) ===========Game Mode On===========",
           getResponse(connM, ":game"), t)
    // test game scenario: time, button press x 2 by 1 client
    assert("(whisper) ======Game reset======",
           getResponse(connM, ":reset"), t)
    getResponse(connM, ":time 10")
    data := getResponse(conn1, "\n")
    assert("(broadcast) Team2, your answer?", data, t)
    // make sure if other player types the answer it won't be accepted
    assert("(whisper) You can't chat right now!",
           getResponse(conn2, "Sorry for inconvenience"), t)
    // make sure other player can't press the button before some answer is given
    // assert("(whisper) You can't press button now", 
    //       getResponse(conn2, "\n"), t)
    data = getResponse(conn1, "42")
    assert("(broadcast) [Team2] 42", data, t)
    // try press button second time
    data = getResponse(conn1, "\n")
    assert("(whisper) You can't press button now", data, t)
    // second client still can press the button
    data = getResponse(conn2, "\n")
    assert("(broadcast) Team1, your answer?", data, t)
    data = getResponse(conn2, "DO NOT PANIC")
    assert("(broadcast) [Team1] DO NOT PANIC", data, t)
    stopServer(s)
}

func TestTimingIssues(t *testing.T) {
    // false start and timeout
    s, _ := startServer()
    connM := enter("Master", true, t)
    //conn1 := enter("Team2", false, t)
    //conn2 := enter("Team1", false, t)
    enter("Team1g", false, t)
    // enter game mode
    assert("(broadcast) ===========Game Mode On===========",
           getResponse(connM, ":game"), t)
    // Team1 has a false start
    //assert("Teuam1 has a false start!", getResponse(conn2, "\n"), t)
    //assert("(broadcast) ===========5 seconds===========",
    //       getResponse(connM, ":time"), t)
    /*
    // wait for timeout 
    //data := waitForData()
    data := "s"
    assert("(broadcast) ===========Time is Out===========", data, t)
    // game auto reset after timeout, no need to call :reset
    assert("(broadcast) ===========5 seconds===========",
           getResponse(connM, ":time 5"), t)
    data = getResponse(conn1, "\n")
    assert("(broadcast) Team2, your answer?", data, t)
    data = getResponse(conn1, "DO NOT PANIC")
    assert("(broadcast) [Team2] DO NOT PANIC", data, t)
    // make sure no false start occurs
    assert("(whisper) You can't press button now", getResponse(conn1, "42"), t)
    */
    stopServer(s)
}

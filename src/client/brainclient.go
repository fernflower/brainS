package client


import ("net" 
        "fmt"
        "bufio"
        "os"
        "settings"
        "time"
        "utils")

func readData(reader *bufio.Reader, ch chan string, err chan error) {
    for {
        line, err := reader.ReadString(settings.EOL)
        utils.ProcError(err)
        ch <- line
    }
}

func StartClient(server string, port int) {
    fmt.Println("Launching Brain Client...")
    conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", server, port))
    utils.ProcError(err)
    chReceive := make(chan string)
    chSend := make(chan string)
    errCh := make(chan error)
    // shellInvite := ">"
    // read data goroutine
    go readData(bufio.NewReader(conn), chReceive, errCh)
    go readData(bufio.NewReader(os.Stdin), chSend, errCh)
    ticker := time.Tick(time.Second)
    for {
        select {
        case data := <-chReceive:
            fmt.Println(data)
        case data := <-chSend:
            if data != "\n"{
                fmt.Fprintf(conn, data)
            }
        case err := <-errCh:
            utils.ProcError(err)
        case <- ticker:
        }
    }
}

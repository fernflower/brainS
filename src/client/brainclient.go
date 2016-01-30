package client


import ("net" 
        "fmt"
        "bufio"
        "os"
        "time"
        "utils")


func StartClient(server string, port int) {
    fmt.Println("Launching Brain Client...")
    conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", server, port))
    utils.ProcError(err)
    chReceive := make(chan string)
    chSend := make(chan string)
    errCh := make(chan error)
    // shellInvite := ">"
    // read data goroutine
    go utils.ReadData(bufio.NewReader(conn), chReceive, errCh)
    go utils.ReadData(bufio.NewReader(os.Stdin), chSend, errCh)
    ticker := time.Tick(time.Second)
    for {
        select {
        case data := <-chReceive:
            fmt.Println(data)
        case data := <-chSend:
            // make sure plain '\n' can be sent
            fmt.Fprintf(conn, data)
        case err := <-errCh:
            utils.ProcError(err)
        case <- ticker:
        }
    }
}

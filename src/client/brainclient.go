package client


import ("net" 
        "fmt"
        "bufio"
        "os"
        "utils")


func StartClient(server string, port int) {
    fmt.Println("Launching Brain Client...")
    conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", server, port))
    utils.ProcError(err)
    for {
        fmt.Print(">")
        // read stdin
        reader := bufio.NewReader(os.Stdin)
        text, err := reader.ReadString('\n')
        utils.ProcError(err)
        // check for empty message
        if text == "\n" {
            continue
        }
        // send message to server
        fmt.Fprintf(conn, text + "\n")
        // wait for a reply
        msg, err := bufio.NewReader(conn).ReadString('\n')
        utils.ProcError(err)
        fmt.Println("(Server says):" + msg)
    }
}

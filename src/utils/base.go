package utils


import ("bufio"
        "fmt"
        "os"
        "settings"
    )

func ProcError(err error) {
    if err != nil {
        // FIXME
        fmt.Println(err)
        os.Exit(1)
    }
}

func ReadData(reader *bufio.Reader, ch chan string, err chan error) {
    for {
        line, e := reader.ReadString(settings.EOL)
        if e != nil {
            err <- e
        } else {
            ch <- line
        }
    }
}

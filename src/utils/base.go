package utils


import ("os"
        "fmt")


func ProcError(err error) {
    if err != nil {
        // FIXME
        fmt.Println(err)
        os.Exit(1)
    }
}



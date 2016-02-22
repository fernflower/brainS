package main


import (
    "server"
    "settings"
)

func main() {
    s := server.NewServer(settings.SERVER, settings.PORT, nil)
    s.Start()
}

package main


import ("server"
        "settings")

func main() {
   server.StartServer(settings.SERVER, settings.PORT) 
}

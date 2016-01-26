package main

import ("client"
        "settings")


func main(){
    client.StartClient(settings.SERVER, settings.PORT)
}

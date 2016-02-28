package server


type Message struct {
    Name string
    Text string
    State string
    Type string
}

type WhoamiMessage struct {
    Name string
    IsMaster bool
    Type string
}

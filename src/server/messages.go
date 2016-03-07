package server

import (
    "encoding/json"
)


type Message struct {
    Type string
    Name string
    Text string
    MasterName string
    State string
}

func (m *Message) ToString() string {
    b, err := json.Marshal(m)
    if err != nil {
        return ""
    }
    return string(b)
}

func FromString(text string) *Message {
    var m Message
    if json.Unmarshal([]byte(text), &m) != nil {
        return nil
    }
    return &m
}

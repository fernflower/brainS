package server

import (
    "encoding/json"
)


type Message struct {
    Type string
    Name string
    Text string
    MasterName string
    // state of the game
    State string
    // action to perform on client side
    Action string
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

func FormMessage(data string) Message {
    /* Forms a Message object from string data.
       If data is json then field values are taken from data,
       otherwise Text field is set to data and other fields are
       set to typical default: system type, author Brain Bot.
    */
    var s map[string] interface{}
    var msg Message
    if json.Unmarshal([]byte(data), &s) != nil {
        // could not unmarshal
        s = make(map[string] interface{})
    }
    fetch := func (key string, def string) string {
        v, ok := s[key]
        if !ok {
            return def
        } else {
            vstr, _ := v.(string)
            return vstr
        }
    }
    msg = Message{Type: fetch("Type", "plain"),
                  Name: fetch("Name", "Brain Bot"),
                  Text: fetch("Text", data),
                  MasterName: fetch("MasterName", ""),
                  State: fetch("State", ""),
                  Action: fetch("Action", "")}
    return msg
}


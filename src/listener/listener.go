package listener

import (
    "errors"
    "net"
    "time"
)

var StoppedError = errors.New("Listener stopped")

type StoppableListener struct {
    *net.TCPListener
    stop chan int
}

func New(l net.Listener) (*StoppableListener, error) {
    tcpL, ok := l.(*net.TCPListener)

    if !ok {
        return nil, errors.New("Cannot wrap listener")
    }

    retval := &StoppableListener{tcpL, make(chan int)}
    return retval, nil
}

func (sl *StoppableListener) Accept() (net.Conn, error) {
    for {
        //Wait up to one second for a new connection
        sl.SetDeadline(time.Now().Add(time.Second))

        newConn, err := sl.TCPListener.Accept()

        //Check for the channel being closed
        select {
        case <-sl.stop:
            return nil, StoppedError
        default:
            //If the channel is still open, continue as normal
        }

        if err != nil {
            netErr, ok := err.(net.Error)

            //If this is a timeout, then continue to wait for
            //new connections
            if ok && netErr.Timeout() && netErr.Temporary() {
                continue
            }
        }
        return newConn, err
    }
}

func (sl *StoppableListener) Stop() {
    close(sl.stop)
    defer sl.TCPListener.Close()
}

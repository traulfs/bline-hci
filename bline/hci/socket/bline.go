package socket

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

// Socket implements a HCI User Channel as ReadWriteCloser.
type Socket struct {
	fd int
	//conn   net.Conn
	//tdPut  chan DataTsb

	//tdDone chan struct{}
	closed chan struct{}
	rmu    sync.Mutex
	wmu    sync.Mutex
}

var tdPut chan DataTsb
var tdGet chan DataTsb
var tdDone chan struct{}
var payloadGet = make(map[byte]chan []byte)

func TsbInit(url string) error {
	conn, err := net.Dial("tcp", url)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("client connected to tcp://%s \n", url)
	tdPut = PutData(conn)
	tdGet, tdDone = GetData(conn)
	TsbServer(3000)
	go func() {
		for {
			select {
			case <-tdDone:
				fmt.Printf("client connection closed!")
				return
			case td := <-tdGet:
				if td.Typ[0] == TypHci {
					if payloadGet[td.Ch[0]] != nil {
						payloadGet[td.Ch[0]] <- td.Payload
					} else {
						//fmt.Printf("tsb channel not initialized: ch: %x, typ: %x payload: % x\n", td.Ch, td.Typ, td.Payload)
					}
				} else {
					TsbOut <- td
					if td.Typ[0] != TypUnknown {
						fmt.Printf("Unexpected tsb-packet: ch: %x, typ: %x payload: % x\n", td.Ch, td.Typ, td.Payload)
					} else {
						fmt.Printf("Anchor: %2d says: %s\n", td.Ch[0]/5, td.Payload)
					}
				}
			}
		}
	}()
	return nil
}

// NewSocket returns a HCI User Channel of specified device id.
// If id is -1, the first available HCI device is returned.
func NewSocket(id int) (*Socket, error) {
	//fmt.Printf("HCI-id: %d\n", id)
	payloadGet[byte(id*5+1)] = make(chan []byte, 100)
	return &Socket{fd: id, closed: make(chan struct{})}, nil
}

func (s *Socket) Read(p []byte) (int, error) {
	s.rmu.Lock()
	defer s.rmu.Unlock()
	for {
		select {
		case <-s.closed:
			return 0, io.EOF
		case payload := <-payloadGet[byte(s.fd*5+1)]:
			n := copy(p, payload)
			return n, nil
		}
	}
}

func (s *Socket) Write(p []byte) (int, error) {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	tdPut <- DataTsb{Ch: []byte{byte(s.fd*5 + 1)}, Typ: []byte{0x15}, Payload: p}
	return len(p), nil
}

func (s *Socket) Close() error {
	//fmt.Printf("Close called\n")
	close(s.closed)
	s.Write([]byte{0x01, 0x09, 0x10, 0x00}) // no-op command to wake up the Read call if it's blocked
	delete(payloadGet, byte(s.fd*5+1))
	s.rmu.Lock()
	defer s.rmu.Unlock()
	return nil
}

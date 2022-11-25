package socket

import (
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/traulfs/tsb"
)

type BeaconLine struct {
	name string
	url  string
	//tsbPort    int
	anchors    int
	conn       net.Conn
	tdPut      chan tsb.TsbData
	tdGet      chan tsb.TsbData
	tdDone     chan struct{}
	payloadGet map[byte]chan []byte
}

// Socket implements a HCI User Channel as ReadWriteCloser.
type Socket struct {
	fd     int
	bl     *BeaconLine
	closed chan struct{}
	rmu    sync.Mutex
	wmu    sync.Mutex
}

func NewBeaconLine(name string, url string, anchors int) (*BeaconLine, error) {
	return &BeaconLine{name: name, url: url, anchors: anchors}, nil
}

func (bl *BeaconLine) Name() string {
	return (bl.name)
}

func (bl *BeaconLine) BeaconLineInit(errChan chan []byte) error {
	var err error
	bl.conn, err = net.Dial("tcp", bl.url)
	if err != nil {
		return err
	}
	bl.payloadGet = make(map[byte]chan []byte)
	for i := 1; i <= bl.anchors; i++ {
		bl.payloadGet[byte(i*5+1)] = make(chan []byte, 100)
	}
	fmt.Printf("client connected to tcp://%s \n", bl.url)
	bl.tdPut = tsb.PutData(bl.conn)
	bl.tdGet, bl.tdDone = tsb.GetData(bl.conn)
	//TsbServer(bl.tsbPort)
	go func() {
		for {
			select {
			case <-bl.tdDone:
				fmt.Printf("client connection closed!")
				return
			case td := <-bl.tdGet:
				if td.Typ[0] == tsb.TypHci {
					if bl.payloadGet[td.Ch[0]] != nil {
						bl.payloadGet[td.Ch[0]] <- td.Payload
						//} else {
						//fmt.Printf("tsb channel not initialized: ch: %x, typ: %x payload: % x\n", td.Ch, td.Typ, td.Payload)
					}
				} else {
					TsbOut <- td
					if td.Typ[0] != tsb.TypError {
						s := fmt.Sprintf("Unexpected tsb-packet: ch: %x, typ: %x payload: % x\n", td.Ch, td.Typ, td.Payload)
						errChan <- []byte(s)
					} else {
						s := fmt.Sprintf("Anchor: %2d says: %s\n", td.Ch[0]/5, td.Payload)
						errChan <- []byte(s)
					}
				}
			}
		}
	}()
	return nil
}

// NewSocket returns a HCI User Channel of specified device id.
// If id is -1, the first available HCI device is returned.
func NewSocket(bl *BeaconLine, id int) (*Socket, error) {
	//fmt.Printf("HCI-id: %d\n", id)
	//bl.payloadGet[byte(id*5+1)] = make(chan []byte, 100)
	return &Socket{fd: id, bl: bl, closed: make(chan struct{})}, nil
}

func (s *Socket) Read(p []byte) (int, error) {
	s.rmu.Lock()
	defer s.rmu.Unlock()
	for {
		select {
		case <-s.closed:
			return 0, io.EOF
		case payload := <-s.bl.payloadGet[byte(s.fd*5+1)]:
			n := copy(p, payload)
			return n, nil
		}
	}
}

func (s *Socket) Write(p []byte) (int, error) {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	s.bl.tdPut <- tsb.TsbData{Ch: []byte{byte(s.fd*5 + 1)}, Typ: []byte{0x15}, Payload: p}
	return len(p), nil
}

func (s *Socket) Close() error {
	fmt.Printf("Close called anchor:  %s-%02d\n", s.bl.name, s.fd)
	close(s.closed)
	s.Write([]byte{0x01, 0x09, 0x10, 0x00}) // no-op command to wake up the Read call if it's blocked
	s.rmu.Lock()
	delete(s.bl.payloadGet, byte(s.fd*5+1))
	defer s.rmu.Unlock()
	return nil
}

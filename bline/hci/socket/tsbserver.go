package socket

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

var clientRW map[io.ReadWriter]byte
var TsbOut chan DataTsb
var TsbIn chan DataTsb

//var TsbDone chan struct{}

func TsbServer(port int) {
	fmt.Printf("tsb server listen to port %d\n", port)
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}
	TsbIn = make(chan DataTsb, 100)
	TsbOut = make(chan DataTsb, 100)
	//TsbDone = make(chan struct{})
	clientRW = make(map[io.ReadWriter]byte)
	go acceptConnection(ln)

	exitSignalCh := make(chan os.Signal)
	signal.Notify(exitSignalCh, os.Interrupt)
	signal.Notify(exitSignalCh, syscall.SIGTERM)
	go func() {
		for {
			select {
			case td := <-TsbOut:
				{
					out := CobsEncode(Encode(td))
					for rw := range clientRW {
						rw.Write(out)
					}
					if Verbose {
						fmt.Printf("TSB-Write: Ch: 0x%X Typ: 0x%X Payload 0x% X\n", td.Ch, td.Typ, td.Payload)
					}
				}
			case <-exitSignalCh:
				{
					//TsbDone <- struct{}{}
					fmt.Printf("TSB terminates!\n")
					os.Exit(0)
					break
				}
			}
		}
	}()
}

func acceptConnection(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn io.ReadWriter) {
	if Verbose {
		fmt.Printf("%v: Got incoming TSB connection\n", conn)
	}
	clientRW[conn] = 0
	done := getTsbData(conn)
	<-done
	delete(clientRW, conn)
	if Verbose {
		fmt.Printf("%v: TSB Connection closed\n", conn)
	}
}

func getTsbData(r io.Reader) chan struct{} {
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 1000000)
		cb := new(bytes.Buffer)
		for {
			n, err := r.Read(buf)
			if err != nil {
				/*
					if err != io.EOF {
						// log.Fatal(err) funktioniert nicht unter Windows
					}
				*/
				break
			}
			for p := 0; p < n; p++ {
				cb.WriteByte(buf[p])
				if buf[p] == 0x00 {
					packet := CobsDecode(cb.Bytes())
					if len(packet) >= 4 {
						td, err := Decode(packet)
						if err != nil {
							log.Print(err)
						} else {
							TsbIn <- td
						}
					} else {
						fmt.Printf("TSB-Read: Wrong cobs packet! packet= % X, buf= % X\n", packet, cb.Bytes())
					}
					cb.Reset()
				}
			}
		}
		done <- struct{}{}
	}()
	return done
}

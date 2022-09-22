/*
Package tsb implements the encoding and decoding of tsb protocol.
It defines the types of tsb.
*/
package socket

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
)

// Data implements the tsb data structure
type DataTsb struct {
	Ch      []byte
	Typ     []byte
	Payload []byte
}

// Channel defintions
const (
	Buflen int = 1000

	TypUnused     byte = 0x00
	TypRaw        byte = 0x01
	TypText       byte = 0x02
	TypEnvelope   byte = 0x05
	TypBline      byte = 0x11
	TypBline2     byte = 0x12
	TypHci        byte = 0x15
	TypCWB        byte = 0x16
	TypCoap       byte = 0x21
	TypCbor       byte = 0x3C
	TypCan        byte = 0x41
	TypModbus     byte = 0x51
	TypUart       byte = 0x52
	TypI2c        byte = 0x53
	TypSpi        byte = 0x54
	TypPort       byte = 0x55
	TypLog        byte = 0x56
	TypError      byte = 0x57
	TypSenmlJSON  byte = 0x6E
	TypSensmlJSON byte = 0x6F
	TypSenmlCbor  byte = 0x70
	TypSensmlCbor byte = 0x71
	TypSenmlExi   byte = 0x72
	TypSensmlExi  byte = 0x73
	TypInflux     byte = 0x75
	TypUnknown    byte = 0x7f
)

// TypLabel maps a string to each type
var TypLabel = map[byte]string{
	TypRaw:        "raw",
	TypText:       "text",
	TypEnvelope:   "envelope",
	TypBline:      "bline",
	TypBline2:     "bline2",
	TypHci:        "hci",
	TypCWB:        "CWB",
	TypCoap:       "coap",
	TypCbor:       "cbor",
	TypCan:        "can",
	TypModbus:     "modbus",
	TypUart:       "uart",
	TypI2c:        "i2c",
	TypSpi:        "spi",
	TypPort:       "port",
	TypSenmlJSON:  "senml_json",
	TypSensmlJSON: "sensml_json",
	TypSenmlCbor:  "senml_cbor",
	TypSensmlCbor: "sensml_cbor",
	TypSenmlExi:   "senml_exi",
	TypSensmlExi:  "sensml_exi",
	TypInflux:     "influx",
	TypError:      "error",
	TypUnknown:    "unknown",
}

// Verbose is a switch for more debug outputs
var Verbose bool

// Channel2Bytes converts a channel string in a []byte
// Example: "3.4.5" -> 0x83,0x84,0x05
func Channel2Bytes(ch string) []byte {
	buf := new(bytes.Buffer)
	if len(ch) < 1 {
		buf.WriteByte(0)
		return buf.Bytes()
	}
	routes := strings.Split(ch, ".")
	for i := 0; i < len(routes); i++ {
		channel, err := strconv.Atoi(routes[i])
		if err != nil {
			log.Printf("Invalid channel string: %s", ch)
		}
		if i < len(routes)-1 {
			buf.WriteByte(byte(channel + 128))
		} else {
			buf.WriteByte(byte(channel))
		}
	}
	return buf.Bytes()
}

// TEncode encodes tsb
func Encode(td DataTsb) []byte {
	buf := new(bytes.Buffer)
	//buf.Write(Channel2Bytes(td.Ch))
	//buf.WriteByte(GetTyp(td.Typ))
	buf.Write(td.Ch)
	buf.Write(td.Typ)
	buf.Write(td.Payload)
	crc := checkSum(buf.Bytes())
	buf.WriteByte(byte(crc & 0xff))
	buf.WriteByte(byte(crc >> 8))
	return buf.Bytes()
}

// Decode encodes tsb
func Decode(packet []byte) (DataTsb, error) {
	var c, t int
	td := DataTsb{}
	for c = 0; packet[c] > 127; c++ {
	}
	c++
	td.Ch = packet[0:c]
	for t = c; packet[t] > 127; t++ {
	}
	t++
	td.Typ = packet[c:t]
	td.Payload = packet[t : len(packet)-2]
	crc := checkSum(packet[0 : len(packet)-2])
	if byte(crc>>8) != packet[len(packet)-1] || byte(crc&0xff) != packet[len(packet)-2] {
		fmt.Printf("TSB-Read:\tCrc error! packet= % X, crc=% X\n", packet, crc)
	} else {
		if Verbose {
			fmt.Printf("TSB-Read:  Ch: 0x%X Typ: 0x%X Payload 0x% X\n", td.Ch, td.Typ, td.Payload)
		}
	}
	return td, nil
}

// CobsEncode implements the cobs algorithmus
func CobsEncode(p []byte) []byte {
	buf := new(bytes.Buffer)
	writeBlock := func(p []byte) {
		buf.WriteByte(byte(len(p) + 1))
		buf.Write(p)
	}
	for _, ch := range bytes.Split(p, []byte{0}) {
		for len(ch) > 0xfe {
			writeBlock(ch[:0xfe])
			ch = ch[0xfe:]
		}
		writeBlock(ch)
	}
	buf.WriteByte(0)
	return buf.Bytes()
}

// CobsDecode implements the cobs algorithmus
func CobsDecode(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	buf := new(bytes.Buffer)
	for n := b[0]; n > 0; n = b[0] {
		if int(n) >= len(b) {
			return nil
		}
		buf.Write(b[1:n])
		b = b[n:]
		if n < 0xff && b[0] > 0 {
			buf.WriteByte(0)
		}
	}
	return buf.Bytes()
}

// GetData reads tsb data from io.Reader and puts it in a channel
func GetData(r io.Reader) (chan DataTsb, chan struct{}) {
	c := make(chan DataTsb, 100)
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
							c <- td
						}
					} else {
						fmt.Printf("TSB-Read: Wrong cobs packet! packet= %X, buf= %X\n", packet, cb.Bytes())
					}
					cb.Reset()
				}
			}
		}
		done <- struct{}{}
	}()
	return c, done
}

// PutData reads tsb data from a channel and writes it to the io.Writer
func PutData(w io.Writer) chan DataTsb {
	c := make(chan DataTsb, 100)
	go func() {
		for {
			td := <-c
			out := CobsEncode(Encode(td))
			_, err := w.Write(out)
			if Verbose {
				fmt.Printf("TSB-Write: Ch: 0x%X Typ: 0x%X Payload 0x% X\n", td.Ch, td.Typ, td.Payload)
			}
			if err != nil {
				log.Fatal(err)
			}
		}
	}()
	return c
}

// GetTypList makes a string of all available Typs
func GetTypList() string {
	var s string
	for i, name := range TypLabel {
		s += fmt.Sprintf("\n\t0x%2X: %s", int(i), name)
	}
	return s
}

/*
// GetTyp return a type from his typename
func GetTyp(typename string) byte {
	for i, name := range TypLabel {
		if typename == name {
			return i
		}
	}
	return TypError
}

// ChannelSplit gives the first root as int and remains the following route
func ChannelSplit(ch *string) (n int) {
	chs := Channel2Bytes(*ch)
	n = int(chs[0])
	if n > 127 {
		n -= 128
		var i int
		*ch = ""
		for i = 1; chs[i] > 127 && len(chs) > i; i++ {
			*ch += fmt.Sprintf("%d.", chs[i]-128)
		}
		*ch += fmt.Sprintf("%d", chs[i])
	} else {
		*ch = "0"
	}
	return n
}
*/

package main

import (
	"net"
	"golang.org/x/net/ipv4"
	"fmt"
	"errors"
	"time"
	"sync"
	"strconv"
)

func multicast_writer(outbound chan string, p *ipv4.PacketConn, dst *net.UDPAddr, err_out *error) {
	for {
		msg := <- outbound
		if _, err := p.WriteTo([]byte(msg), nil, dst); err != nil {
			*err_out = errors.New("Failed to WriteTo")
			fmt.Println(*err_out)
			return
		} else {
			fmt.Println("Sent: ", msg)
		}	
	}
}

func multicast_reader(inbound chan string, p *ipv4.PacketConn, group net.IP, b []byte, err_out *error) {
	for {
		p.SetReadDeadline(time.Now().Add(10 * time.Second))
		fmt.Println("Attempting to Read...")
		n, cm, _, err := p.ReadFrom(b)
		if err != nil {
			if netOpError, ok := err.(*net.OpError); ok {
				if netOpError.Timeout() {
					// Pass, no packet available
					fmt.Println("ReadFrom Timed Out at: ", time.Now())
				} else {
					*err_out = errors.New("Failed to ReadFrom")
					fmt.Println(*err_out)
					return
				}
			} else { // Failed to convert to net.OpError
				*err_out = errors.New("Failed to ReadFrom with non net.OpError")
				fmt.Println(*err_out)
				return
			}
		} else { // err is nil, no errors
			if cm.Dst.IsMulticast() && cm.Dst.Equal(group) {
				msg := string(b[:n])
				inbound <- msg
				fmt.Println("Recieved: ", msg)
			}	
		}
	}
}

func multicast(inbound chan string, outbound chan string, err_out *error, writer_error *error, reader_error *error) {
	en0, err := net.InterfaceByName("en0")
	if err != nil {
		*err_out = errors.New("Failed to get InterfaceByName")
		fmt.Println(*err_out)
		return
	}

	group := net.IPv4(224, 0, 0, 250)

	c, err := net.ListenPacket("udp4", "0.0.0.0:1024")
	if err != nil {
		*err_out = errors.New("Failed to get ListenPacket")
		fmt.Println(*err_out)
		return
	}
	defer c.Close()

	p := ipv4.NewPacketConn(c)
	if err := p.JoinGroup(en0, &net.UDPAddr{IP: group}); err != nil {
		*err_out = errors.New("Failed to JoinGroup")
		fmt.Println(*err_out)
		return
	}

	if err := p.SetControlMessage(ipv4.FlagDst, true); err != nil {
		*err_out = errors.New("Failed to SetControlMessage")
		fmt.Println(*err_out)
		return
	}

	if err := p.SetMulticastInterface(en0); err != nil {
		*err_out = errors.New("Failed to SetMulticastInterface")
		fmt.Println(*err_out)
		return
	}

	p.SetMulticastTTL(2)

	dst := &net.UDPAddr{IP: group, Port:1024}
	b := make([]byte, 1024)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		multicast_writer(outbound, p, dst, writer_error)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		multicast_reader(inbound, p, group, b, reader_error)
	}()

	wg.Wait()
}

func inbound_listener(inbound chan string) {
	for {
		select {
		case msg := <- inbound:
			fmt.Println("Recieved From Queue: ", msg)
		}
	}
}

func outbound_writer(outbound chan string) {
	var i int64 = 0
	for {
		msg := strconv.FormatInt(i, 10)
		fmt.Println("Enqueing To Send: ", msg)
		outbound <- msg
		time.Sleep(100 * time.Millisecond)
		i++
	}
}

func main() {
	inbound := make(chan string)
	outbound := make(chan string)
	var multicast_error error = nil
	var writer_error error = nil
	var reader_error error = nil
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		multicast(inbound, outbound, &multicast_error, &writer_error, &reader_error)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		inbound_listener(inbound)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		outbound_writer(outbound)
	}()

	wg.Wait()
}
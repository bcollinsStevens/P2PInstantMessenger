package main

import (
	"net"
	"fmt"
	"log"
	"time"
	"github.com/marcusolsson/tui-go"
)

const NETWORK = "udp"
const SERVICE_PORT = 1024
const READ_BUFFER_SIZE = 256

func UDPAddrEqual(a, b *net.UDPAddr) bool {
	return (a.IP.Equal(b.IP) && (a.Port == b.Port))
}

func main() {
	var selectedInterface *net.Interface
	{ // Interface Selection
		var err error
		interfaces, err := net.Interfaces()
		if err != nil {
			log.Fatal(err)
		}
		availableInterfaceIndexes := []int{}
		flagUpAndMulticast := net.FlagUp | net.FlagMulticast
		for i, iface := range interfaces {
			if (iface.Flags & flagUpAndMulticast) == flagUpAndMulticast {
				availableInterfaceIndexes = append(availableInterfaceIndexes, i)
			}
		}
		
		if len(availableInterfaceIndexes) == 0 {
			fmt.Println("No Available Network Interfaces For Multicast")
			return;
		}

		endAvailableInterfacesIdx := len(availableInterfaceIndexes) - 1
		var user_i int
		for done:=false;!done; {
			for i, idx := range availableInterfaceIndexes {
				fmt.Print(fmt.Sprintf("(%d, %s)", i, interfaces[idx].Name))
				if i < endAvailableInterfacesIdx {
					fmt.Print(", ")
				}
			}
			fmt.Println()
			fmt.Print(fmt.Sprintf("Select a Network Interface by index (index, name): "))
			fmt.Scanf("%d", &user_i)
			if (0 <= user_i) && (user_i <= endAvailableInterfacesIdx) {
				done = true
			} else {
				fmt.Println(fmt.Sprintf("Please enter a number in the range [0-%d]", endAvailableInterfacesIdx))
			}
		}
		selectedInterface, err = net.InterfaceByName(interfaces[availableInterfaceIndexes[user_i]].Name)
		if err != nil {
			log.Fatal(err)
		}
	}

	var selectedGroupAddress *net.UDPAddr
	{ // Group Address Selection
		var user_group byte
		for done := false; !done; {
			fmt.Print("Select a Multicast Group ID in the range [151-250]: ")
			fmt.Scanf("%d", &user_group)
			if (151 <= user_group) && (user_group <= 250) {
				done = true
			}
		}
		var err error
		selectedGroupAddress, err = net.ResolveUDPAddr(NETWORK, fmt.Sprintf("224.0.0.%d:%d", user_group, SERVICE_PORT))
		if err != nil {
			log.Fatal(err)
		}
	}
	
	var readConnection *net.UDPConn
	{ // Read Connection Setup
		var err error
		readConnection, err = net.ListenMulticastUDP(NETWORK, selectedInterface, selectedGroupAddress)
		if err != nil {
			log.Fatal(err)
		}
	}

	var writeConnection *net.UDPConn
	var localAddr *net.UDPAddr
	{ // Write Connection Setup
		var err error
		writeConnection, err = net.DialUDP(NETWORK, nil, selectedGroupAddress)
		if err != nil {
			log.Fatal(err)
		}
		localAddr, err = net.ResolveUDPAddr(writeConnection.LocalAddr().Network(), writeConnection.LocalAddr().String())
		if err != nil {
			log.Fatal(err)
		}
	}

	type Message struct {
		src *net.UDPAddr
		msg string
	}
	readChannel := make(chan Message)
	go func() { // spawn reader that pushes to readChannel
		buf := make([]byte, READ_BUFFER_SIZE)
		readConnection.SetReadBuffer(READ_BUFFER_SIZE)
		for {
			n_bytes, src, err := readConnection.ReadFromUDP(buf)
			if err != nil {
				log.Fatal(err)
			}
			readChannel <- Message{src:src, msg:string(buf[:n_bytes])}
		}
	}()

	writeChannel := make(chan string)
	go func() { // spawn writer that pulls from writeChannel
		for {
			message := <- writeChannel
			writeConnection.Write([]byte(message))
		}
	}()

	var ui tui.UI
	var history *tui.Box
	{ // Setup but DONT START ui
		history = tui.NewVBox()
		historyScroll := tui.NewScrollArea(history)
		historyScroll.SetAutoscrollToBottom(true)
		historyBox := tui.NewVBox(historyScroll)
		historyBox.SetBorder(true)

		input := tui.NewEntry()
		input.SetFocused(true)
		input.SetSizePolicy(tui.Expanding, tui.Maximum)
		inputBox := tui.NewHBox(input)
		inputBox.SetBorder(true)
		inputBox.SetSizePolicy(tui.Expanding, tui.Maximum)

		input.OnSubmit(func(e *tui.Entry) {
			writeChannel <- e.Text()
			input.SetText("")
		})

		chat := tui.NewVBox(historyBox, inputBox)
		chat.SetSizePolicy(tui.Expanding, tui.Expanding)

		var err error
		ui, err = tui.New(chat)
		if err != nil {
			log.Fatal(err)
		}

		theme := tui.NewTheme()
		theme.SetStyle("label.important", tui.Style{Bold: tui.DecorationOn, Underline: tui.DecorationOn})

		ui.SetTheme(theme)

		ui.SetKeybinding("Esc", func() { ui.Quit() })
	}

	var appendHistory func(src string, msg string, from_me bool)
	{ // Setup appendHistory function
		appendHistory = func(src string, msg string, from_me bool) {
			source_label := tui.NewLabel(fmt.Sprintf("<%s>", src))
			if from_me {
				source_label.SetStyleName("important")
			}
			history.Append(tui.NewHBox(tui.NewLabel(time.Now().Format("15:04")), tui.NewPadder(1, 0, source_label), tui.NewLabel(msg), tui.NewSpacer()))
		}
	}

	

	go func() { // Spawn goroutine that pulls from the readChannel and appends to the chat history
		for {
			message := <- readChannel
			ui.Update(func() {
				appendHistory(message.src.String(), message.msg, UDPAddrEqual(message.src, localAddr))
			})
		}
	}()

	if err := ui.Run(); err != nil {
		log.Fatal(err)
	}
}
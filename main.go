package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
)

// Exit codes
const (
	_ int = iota + 2
	macAddressNotEUI48Error
	macAddressNotInformedError
	networkAdapterAddressesFetchingError
	networkAdapterFetchingError
	networkAdaptersFetchingError
	notAllWOLPayloadBytesSentError
	udpConnectionError
	wolPayloadSendingError
)

const (
	eui48Length      int = 6
	wakeOnLanUDPPort int = 7
	payloadLength    int = eui48Length + (16 * eui48Length)
)

var programFlags struct {
	listNetworkAdapters bool
	macAddress          net.HardwareAddr
	networkAdapterName  string
}

type wolPayload [payloadLength]byte

func addressFromNetworkAdapter(networkAdapter net.Interface) *net.UDPAddr {
	networkAdapterAddresses, err := networkAdapter.Addrs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "The following error occurred when fetching the addresses of the network adapter of index %d: %s\n",
			networkAdapter.Index,
			err.Error())

		os.Exit(networkAdapterAddressesFetchingError)
	}

	for _, networkAdapterAddress := range networkAdapterAddresses {
		switch ip := networkAdapterAddress.(type) {
		case *net.IPNet:
			ipv4 := ip.IP.To4()
			if ipv4 == nil || ipv4.IsLoopback() {
				continue
			}

			return &net.UDPAddr{IP: ipv4, Port: wakeOnLanUDPPort}
		}
	}

	return nil
}

func addressFromNetworkAdapterName(networkAdapterName string) *net.UDPAddr {
	networkAdapter, err := net.InterfaceByName(networkAdapterName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "The following error occurred when fetching the network adapter named %s: %s\n",
			networkAdapterName,
			err.Error())

		os.Exit(networkAdapterFetchingError)
	}

	return addressFromNetworkAdapter(*networkAdapter)
}

func checkParsedMACAddress() {
	if programFlags.listNetworkAdapters {
		return
	}

	if programFlags.macAddress == nil {
		fmt.Fprintln(os.Stderr, "No MAC address informed. See program usage (-h flag).")

		os.Exit(macAddressNotInformedError)
	}

	if len(programFlags.macAddress) != eui48Length {
		fmt.Fprintln(os.Stderr, "MAC address must be an EUI-48 identifier.")

		os.Exit(macAddressNotEUI48Error)
	}
}

func closeUDPConnection(udpConnection *net.UDPConn) {
	udpConnection.Close()
}

func createWOLPayload() wolPayload {
	payload := wolPayload{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

	payloadIndex := 6

	for i := 0; i < 16; i++ {
		for j := 0; j < eui48Length; j++ {
			payload[payloadIndex] = programFlags.macAddress[j]

			payloadIndex++
		}
	}

	return payload
}

func listNetworkAdapters() {
	networkAdapters, err := net.Interfaces()
	if err != nil {
		fmt.Fprintln(os.Stderr, "The following error occurred when fetching the network adapters of the system: "+err.Error())

		os.Exit(networkAdaptersFetchingError)
	}

	for _, networkAdapter := range networkAdapters {
		networkAdapterAddress := addressFromNetworkAdapter(networkAdapter)

		if networkAdapterAddress != nil {
			fmt.Printf("Local Address: %-15s | Name: %s\n",
				networkAdapterAddress.IP.String(),
				networkAdapter.Name)
		}
	}
}

func openUDPConnection() *net.UDPConn {
	var (
		localAddress  = addressFromNetworkAdapterName(programFlags.networkAdapterName)
		remoteAddress = &net.UDPAddr{IP: net.IP{255, 255, 255, 255}, Port: wakeOnLanUDPPort}
	)

	udpConnection, err := net.DialUDP("udp4", localAddress, remoteAddress)
	if err != nil {
		fmt.Fprintf(os.Stderr, "The following error occurred when trying to connect to the remote address %s from the local address %s: %s\n",
			remoteAddress.String(),
			localAddress.String(),
			err.Error())

		os.Exit(udpConnectionError)
	}

	return udpConnection
}

func parseMACAddressFlag(macAddress string) error {
	var err error

	programFlags.macAddress, err = net.ParseMAC(macAddress)
	if err != nil {
		return errors.New("could not parse MAC address")
	}

	return nil
}

func parseProgramFlags() {
	flag.Parse()

	checkParsedMACAddress()
}

func sendWOLPayload(udpConnection *net.UDPConn, payload wolPayload) {
	fmt.Printf("Sending Wake-on-LAN payload to the remote address %s from the local address %s.\n",
		udpConnection.RemoteAddr().String(),
		udpConnection.LocalAddr().String())

	bytesWritten, err := udpConnection.Write(payload[:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "The following error occurred when sending the Wake-on-LAN payload: "+err.Error())

		os.Exit(wolPayloadSendingError)

	} else if bytesWritten == payloadLength {
		fmt.Printf("Wake-on-LAN payload sent.")

	} else {
		fmt.Fprintf(os.Stderr, "Not all %d bytes of the payload were sent to the remote address.\n", payloadLength)

		os.Exit(notAllWOLPayloadBytesSentError)
	}
}

func wakeRemoteComputer() {
	udpConnection := openUDPConnection()

	sendWOLPayload(udpConnection, createWOLPayload())

	defer closeUDPConnection(udpConnection)
}

func init() {
	flag.BoolVar(&programFlags.listNetworkAdapters, "list-network-adapters", false, "lists system network adapters")
	flag.StringVar(&programFlags.networkAdapterName, "network-adapter-name", "", "`name` of the network adapter to be used")

	flag.Func("mac-address", "`mac address` of the computer to be awaken", parseMACAddressFlag)
}

func main() {
	parseProgramFlags()

	if programFlags.listNetworkAdapters {
		listNetworkAdapters()

	} else {
		wakeRemoteComputer()
	}
}

package client

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/google/uuid"
	"github.com/nikochiko/tcpchat/common"
)

func Connect(service string) {
	raddr, err := net.ResolveTCPAddr("tcp4", service)
	common.CheckError(err)

	conn, err := net.DialTCP("tcp", nil, raddr)
	common.CheckError(err)

	log.Printf("Established connection with %s\n", raddr.String())

	defer func() {
		conn.Close()
		log.Printf("Connection with %s closed\n", raddr.String())
	}()

	handleConnection(conn)
}

func handleConnection(conn net.Conn) {
	name := getClientName()

	aboutClient := initialiseSender(name)
	writeJSONTo(conn, aboutClient)

	var err error

	for {
		switch operationType := getOperationType(); strings.ToLower(operationType) {
		case common.CreateOperationType:
			var name string
			fmt.Scanf("%s", &name)
			err = createConversation(conn, name)
		}

		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			break
		}
	}
}

func initialiseSender(name string) *common.ClientAboutMe {
	aboutMe := &common.ClientAboutMe{
		Name: name,
		ID:   uuid.New(),
	}

	return aboutMe
}

func getClientName() (name string) {
	fmt.Print("Enter your chat display name: ")
	fmt.Scan(&name)

	return name
}

func getOperationType() (operationType string) {
	fmt.Print("Enter the operation type to execute: ")
	fmt.Scan(&operationType)

	return operationType
}

func listConversations(conn net.Conn) {
}

func createConversation(conn net.Conn, nickname string) error {
	newConversation := common.Conversation{Nickname: nickname}
	marshaled, err := json.Marshal(newConversation)
	if err != nil {
		return err
	}

	conversationJSON := json.RawMessage(marshaled)

	operation := common.Operation{
		Type:    common.CreateOperationType,
		Message: &conversationJSON,
	}

	err = writeJSONTo(conn, operation)
	if err != nil {
		return err
	}

	response := &common.Response{}
	err = readJSONFrom(conn, response)
	if err != nil {
		return err
	}

	if response.Status == "ok" {
		fmt.Printf("Received OK response: %s\n", string(*response.Message))
	} else if response.Status == "error" {
		err := fmt.Sprintf("got error response from server: %s", response.Error.Message)
		return errors.New(err)
	}

	return nil
}

func writeJSONTo(conn net.Conn, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}

	_, err = conn.Write(append(b, common.EOFBytes...))
	if err != nil {
		return err
	}

	return nil
}

func readJSONFrom(conn net.Conn, v interface{}) error {
	buf := make([]byte, 1024)

	nBytes, err := bufio.NewReader(conn).Read(buf)
	if err != nil {
		return err
	}

	err = json.Unmarshal(buf[:nBytes], v)
	if err != nil {
		return err
	}

	return nil
}

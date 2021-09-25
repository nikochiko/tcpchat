package client

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nikochiko/tcpchat/common"
)

var globalConversations = []*common.Conversation{}

func Connect(service string) {
	raddr, err := net.ResolveTCPAddr("tcp4", service)
	common.CheckError(err)

	conn, err := net.DialTCP("tcp", nil, raddr)
	common.CheckError(err)

	quitConn := make(chan bool)
	go handleConnection(conn, quitConn)

	log.Printf("Established connection with %s\n", conn.RemoteAddr().String())

	for {
		select {
		case <-quitConn:
			conn.Close()
			log.Printf("Connection with %s closed\n", conn.RemoteAddr().String())
			return
		}
	}
}

func handleConnection(conn net.Conn, quitConn chan bool) {
	var err error

	defer func() {
		quitConn <- true
	}()

	name := getClientName()

	aboutClient := initialiseSender(name)
	writeJSONTo(conn, aboutClient)

	quit := make(chan bool)
	go handleIncoming(conn, quit)
	defer func() {
		quit <- true
	}()

	err = listConversations(conn)
	common.CheckError(err)

	for {
		switch operationType := getOperationType(); strings.ToLower(operationType) {
		case common.CreateOperationType:
			var name string
			fmt.Scanf("%s", &name)
			err = createConversation(conn, name)
		case common.SubscribeOperationType:
			var convNickname string
			fmt.Scanf("%s", &convNickname)
			err = subscribe(conn, convNickname)
		case common.MessageOperationType:
			var convNickname string
			fmt.Scanf("%s", &convNickname)
			err = sendMessage(conn, convNickname)
		case common.ListOperationType:
			err = listConversations(conn)
		}

		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			break
		}
	}
}

func handleIncoming(conn net.Conn, quit chan bool) {
	for {
		conn.SetReadDeadline(time.Now().Add(10 * time.Minute))
		select {
		case <-quit:
			return
		default:
			response := common.Response{}

			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			err := readJSONFrom(conn, &response)

			if errors.Is(err, os.ErrDeadlineExceeded) {
				continue
			}
			if err != nil {
				common.CheckError(err)
			}

			if response.Status == "ok" {
				log.Printf("Received OK response: %s\n", string(*response.Message))
			} else if response.Status == "error" {
				err := fmt.Sprintf("got error response from server: %s", response.Error.Message)
				common.CheckErrorAndLog(errors.New(err))
			}

			handleResponse(response)
		}
	}
}

func handleResponse(response common.Response) {
	switch response.OperationType {
	case common.ListOperationType:
		handleListOperationResponse(response.Message)
	case common.MessageOperationType:
		handleMessageOperationResponse(response.Message)
		// ignore in all other cases
	}
}

func handleListOperationResponse(jsonConversations *json.RawMessage) {
	err := json.Unmarshal(*jsonConversations, &globalConversations)
	common.CheckError(err)
}

func handleMessageOperationResponse(jsonMessage *json.RawMessage) {
	message := common.Message{}

	err := json.Unmarshal(*jsonMessage, &message)
	common.CheckError(err)

	fmt.Printf("\n<@%s>: %s\n", message.Sender.Name, message.Text)
}

func listConversations(conn net.Conn) error {
	emptyJSON := json.RawMessage("{}")

	operation := common.Operation{
		Type:    common.ListOperationType,
		Message: &emptyJSON,
	}

	err := writeJSONTo(conn, operation)
	if err != nil {
		return err
	}

	return nil
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

	return nil
}

func subscribe(conn net.Conn, convNickname string) error {
	conversation := common.Conversation{Nickname: convNickname}

	marshaled, err := json.Marshal(conversation)
	if err != nil {
		return err
	}

	conversationJSON := json.RawMessage(marshaled)

	operation := common.Operation{
		Type:    common.SubscribeOperationType,
		Message: &conversationJSON,
	}

	err = writeJSONTo(conn, operation)
	if err != nil {
		return err
	}

	return nil
}

func sendMessage(conn net.Conn, convNickname string) error {
	var text string
	_, err := fmt.Scanf("%s\r", &text)
	if err != nil {
		return err
	}

	fmt.Println(text)
	// message := common.Message{
	// 	ConversationNickname: convNickname,
	// 	SenderName:           senderName,
	// 	Text:                 text,
	// }

	return nil
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

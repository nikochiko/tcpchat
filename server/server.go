package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"

	"github.com/google/uuid"
	"github.com/nikochiko/tcpchat/common"
)

const (
	unmarshalingError = "Error while unmarshaling data. Please check again"
)

var conversationIDs = map[uuid.UUID]bool{}
var conversations = []*common.Conversation{}
var conversationsByNickname = map[string]*common.Conversation{}

var messagesChannel = make(chan common.Message)

// Listen starts listening on the given service ("host:port") for TCP connections
func Listen(service string) error {
	laddr, err := net.ResolveTCPAddr("tcp4", service)
	common.CheckError(err)

	listener, err := net.ListenTCP("tcp", laddr)
	common.CheckError(err)

	fmt.Printf("Started listening on %s\n", laddr)

	// start listening indefinitely
	for {
		conn, err := listener.Accept()
		if err != nil {
			// we don't want to stop the server now, so just log and continue
			log.Printf("Error while accepting connection: %s", err.Error())

			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	connReader := bufio.NewReader(conn)
	request, err := common.ReadUntil(connReader, common.EOFBytes)
	if common.CheckErrorAndLog(err) {
		writeErrorResponse(conn, "Some error occurred")
		return
	}

	operation, err := getOperation(request)
	if common.CheckErrorAndLog(err) {
		writeErrorResponse(conn, err.Error())
		return
	}

	aboutClient, err := ParseClientAboutMe(*operation.Message)
	if common.CheckErrorAndLog(err) {
		writeErrorResponse(conn, err.Error())
		return
	}

	err = sendAboutMeResponse(conn, aboutClient)
	if common.CheckErrorAndLog(err) {
		writeErrorResponse(conn, err.Error())
		return
	}

	log.Printf("New connection received from client: %v\n", aboutClient)

	conversationsToListenOn := map[uuid.UUID]bool{}

	quit := make(chan bool)
	go subscribeToMessages(conn, conversationsToListenOn, quit)
	defer func() {
		quit <- true
	}()

	for {
		request, err := common.ReadUntil(connReader, common.EOFBytes)
		if err == io.EOF {
			log.Printf("connection closed. exiting function\n")
			break
		} else {
			common.CheckErrorAndLog(err)
		}

		operation, err := getOperation(request)
		if common.CheckErrorAndLog(err) {
			writeErrorResponse(conn, err.Error())
			break
		}

		emptyJSON := json.RawMessage("{}")
		var response = &emptyJSON

		switch operation.Type {
		case common.CreateOperationType:
			err = handleCreateConversation(operation)
		case common.SubscribeOperationType:
			err = handleSubscribe(operation, conversationsToListenOn)
		case common.MessageOperationType:
			response, err = handleMessage(operation)
		case common.ListOperationType:
			response, err = handleListConversations(operation)
		}

		if err != nil {
			writeErrorResponse(conn, err.Error())
			break
		}

		err = writeOKResponse(conn, response, operation.Type)

		if err != nil {
			writeErrorResponse(conn, err.Error())
			break
		}
	}

	return
}

func subscribeToMessages(conn net.Conn, conversationsToListenOn map[uuid.UUID]bool, quit chan bool) {
	for {
		select {
		case <-quit:
			return
		case message := <-messagesChannel:
			if conversationsToListenOn[message.Conversation.ID] {
				responseBytes, err := json.Marshal(message)
				if err != nil {
					log.Printf("error while marshaling message: %s\n", err.Error())

					// let's continue listening for other messages
					continue
				}

				responseJSON := json.RawMessage(responseBytes)
				writeOKResponse(conn, &responseJSON, common.MessageOperationType)
			}
		}
	}
}

func sendAboutMeResponse(conn net.Conn, aboutClient *common.ClientAboutMe) error {
	b, err := json.Marshal(aboutClient)
	if err != nil {
		log.Printf("Error: %s\n", err.Error())
		return err
	}

	jsonAboutClient := json.RawMessage(b)

	writeOKResponse(conn, &jsonAboutClient, common.AboutMeOperationType)

	return nil
}

func handleCreateConversation(op *common.Operation) error {
	conversation := &common.Conversation{}

	err := json.Unmarshal(*op.Message, conversation)
	if err != nil {
		log.Printf("Unmarshaling error while parsing Conversation: %s\n", err.Error())
		return errors.New(unmarshalingError)
	}

	conversation.ID = uuid.New()

	if conversation.Nickname == "" {
		conversation.Nickname = strconv.Itoa(len(conversations))
	}

	if _, ok := conversationsByNickname[conversation.Nickname]; ok {
		err := fmt.Sprintf("conversation with nickname '%s' already exists", conversation.Nickname)
		return errors.New(err)
	}

	conversations = append(conversations, conversation)
	conversationIDs[conversation.ID] = true
	conversationsByNickname[conversation.Nickname] = conversation

	return nil
}

func handleListConversations(op *common.Operation) (*json.RawMessage, error) {
	emptyJSON := json.RawMessage("{}")

	conversationsJSON, err := json.Marshal(conversations)
	if err != nil {
		return &emptyJSON, err
	}

	responseMessage := json.RawMessage(conversationsJSON)

	return &responseMessage, err
}

func handleSubscribe(op *common.Operation, conversationsToListenOn map[uuid.UUID]bool) error {
	inputConversation := &common.Conversation{}

	err := json.Unmarshal(*op.Message, inputConversation)
	if err != nil {
		log.Printf("Unmarshaling error while parsing Conversation: %s\n", err.Error())
		return errors.New(unmarshalingError)
	}

	nickname := inputConversation.Nickname
	conversation, ok := conversationsByNickname[nickname]
	if !ok {
		err := fmt.Sprintf("conversation '%s' does not exist", nickname)
		return errors.New(err)
	}

	convID := conversation.ID
	conversationsToListenOn[convID] = true

	return nil
}

func handleMessage(op *common.Operation) (*json.RawMessage, error) {
	message := json.RawMessage("{}")
	convMessage := common.Message{}

	err := json.Unmarshal(*op.Message, &convMessage)
	if err != nil {
		log.Printf("Unmarshaling error while parsing Message: %s\n", err.Error())
		return &message, errors.New(unmarshalingError)
	}

	log.Printf("Got message: %s\n", string(*op.Message))

	messagesChannel <- convMessage

	return &message, nil
}

// ParseClientAboutMe parses the data first sent by Client to introduce themselves
func ParseClientAboutMe(b []byte) (*common.ClientAboutMe, error) {
	aboutClient := &common.ClientAboutMe{ID: uuid.New()}

	log.Printf("got about me: %s\n", string(b))

	err := json.Unmarshal(b, aboutClient)
	if err != nil {
		log.Printf("Unmarshaling error while parsing ClientAboutMe: %s\n", err.Error())
		return aboutClient, errors.New(unmarshalingError)
	}

	return aboutClient, nil
}

func getOperation(b []byte) (*common.Operation, error) {
	operation := &common.Operation{}

	err := json.Unmarshal(b, operation)
	if err != nil {
		log.Printf("Unmarshaling error while parsing Operation: %s\n", err.Error())
		return operation, errors.New(unmarshalingError)
	}

	return operation, nil
}

func writeErrorResponse(conn net.Conn, s string) {
	defer conn.Close()

	errorMessage := common.Error{Message: s}
	response := common.NewResponse()
	response.Status = "error"
	response.Error = &errorMessage

	responseBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("Got another error while writing one error: %s", err.Error())
	}

	conn.Write(responseBytes)
	conn.Write(common.EOFBytes)
	conn.Close()
}

func writeOKResponse(conn net.Conn, message *json.RawMessage, operationType string) error {
	response := common.NewResponse()
	response.Status = "ok"

	if operationType != "" {
		response.OperationType = operationType
	}

	if !bytes.Equal(*message, []byte{}) {
		response.Message = message
	}

	log.Printf("Message: %s\n", string(*message))

	responseBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("Got an error while marshaling an OK response: %s", err.Error())
		err := errors.New("Something went wrong")
		return err
	}

	_, err = conn.Write(append(responseBytes, common.EOFBytes...))
	if err != nil {
		return err
	}

	return nil
}

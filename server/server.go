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
	buf := make([]byte, 512)

	nBytes, err := bufio.NewReader(conn).Read(buf)
	if common.CheckErrorAndLog(err) {
		writeErrorResponse(conn, "Some error occurred")
		return
	}

	aboutClient, err := ParseClientAboutMe(buf[:nBytes])
	if common.CheckErrorAndLog(err) {
		writeErrorResponse(conn, err.Error())
		return
	}

	log.Printf("New connection received from client: %v\n", aboutClient)

	conversationsToListenOn := map[uuid.UUID]bool{}

	for {
		nBytes, err := bufio.NewReader(conn).Read(buf)

		if err == io.EOF {
			log.Printf("connection closed. exiting function\n")
			break
		}

		operation, err := getOperation(buf[:nBytes])
		if common.CheckErrorAndLog(err) {
			writeErrorResponse(conn, err.Error())
			break
		}

		var response *json.RawMessage

		switch operation.Type {
		case common.CreateOperationType:
			response, err = handleCreateConversation(operation)
		case common.SubscribeOperationType:
			response, err = handleSubscribe(operation, conversationsToListenOn)
		}

		if err != nil {
			writeErrorResponse(conn, err.Error())
			break
		}

		err = writeOKResponse(conn, response)

		if err != nil {
			writeErrorResponse(conn, err.Error())
			break
		}
	}

	return
}

func handleCreateConversation(op *common.Operation) (*json.RawMessage, error) {
	message := json.RawMessage("{}")
	conversation := &common.Conversation{}

	err := json.Unmarshal(*op.Message, conversation)
	if err != nil {
		log.Printf("Unmarshaling error while parsing Conversation: %s\n", err.Error())
		return &message, errors.New(unmarshalingError)
	}

	conversation.ID = uuid.New()

	if conversation.Nickname == "" {
		conversation.Nickname = strconv.Itoa(len(conversations))
	}

	if _, ok := conversationsByNickname[conversation.Nickname]; ok {
		err := fmt.Sprintf("conversation with nickname '%s' already exists", conversation.Nickname)
		return &message, errors.New(err)
	}

	conversations = append(conversations, conversation)
	conversationIDs[conversation.ID] = true
	conversationsByNickname[conversation.Nickname] = conversation

	b, err := json.Marshal(conversation)
	if err != nil {
		log.Printf("Marshaling error while creating Conversation{} for returning back: %s\n", err.Error())
		return &message, errors.New("Something went wrong")
	}

	message = json.RawMessage(b)

	return &message, nil
}

func handleSubscribe(op *common.Operation, conversationsToListenOn map[uuid.UUID]bool) (*json.RawMessage, error) {
	message := json.RawMessage("{}")
	inputConversation := &common.Conversation{}

	err := json.Unmarshal(*op.Message, inputConversation)
	if err != nil {
		log.Printf("Unmarshaling error while parsing Conversation: %s\n", err.Error())
		return &message, errors.New(unmarshalingError)
	}

	nickname := inputConversation.Nickname
	conversation, ok := conversationsByNickname[nickname]
	if !ok {
		err := fmt.Sprintf("conversation '%s' does not exist", nickname)
		return &message, errors.New(err)
	}

	convID := conversation.ID
	conversationsToListenOn[convID] = true

	message = json.RawMessage(fmt.Sprintf("listening on conversation '%s'", nickname))

	return &message, nil
}

// ParseClientAboutMe parses the data first sent by Client to introduce themselves
func ParseClientAboutMe(b []byte) (*common.ClientAboutMe, error) {
	aboutClient := &common.ClientAboutMe{}

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

func writeOKResponse(conn net.Conn, message *json.RawMessage) error {
	response := common.NewResponse()
	response.Status = "ok"
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

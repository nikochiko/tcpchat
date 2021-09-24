package server

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"

	"github.com/google/uuid"
	"github.com/nikochiko/tcpchat/common"
)

const (
	UnmarshalingError        = "Error while unmarshaling data. Please check again"
	NewMessageHeader         = "message"
	SubscribeHeader          = "subscribe"
	ListConveresationsHeader = "list"
	NewConversationHeader    = "new"
)

var EOFBytes = []byte("\r\n")

var conversationIDs = map[uuid.UUID]bool{}

// Listen starts listening on the given service ("host:port") for TCP connections
func Listen(service string) error {
	laddr, err := net.ResolveTCPAddr("tcp4", service)
	common.CheckError(err)

	listener, err := net.ListenTCP("tcp", laddr)
	common.CheckError(err)

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

	// conversationsToListenOn := map[uuid.UUID]bool{}

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
		// case common.SubscribeOperationType:
		// 	response, err = handleSubscribeToConversation(operation)
		}

		err = writeOKResponse(conn, response)
		if err != nil {
			writeErrorResponse(conn, err.Error())
			return
		}
	}

	return
}

func handleCreateConversation(op *common.Operation) (*json.RawMessage, error) {
	response := &json.RawMessage{}
	conversation := &common.Conversation{}

	err := json.Unmarshal(*op.Message, conversation)
	if err != nil {
		log.Printf("Unmarshaling error while parsing Conversation: %s\n", err.Error())
		return response, errors.New(UnmarshalingError)
	}

	conversationIDs[conversation.ID] = true

	// empty response is fine here, nothing to give to client
	return response, nil
}

// ParseClientAboutMe parses the data first sent by Client to introduce themselves
func ParseClientAboutMe(b []byte) (*common.ClientAboutMe, error) {
	aboutClient := &common.ClientAboutMe{}

	err := json.Unmarshal(b, aboutClient)
	if err != nil {
		log.Printf("Unmarshaling error while parsing ClientAboutMe: %s\n", err.Error())
		return aboutClient, errors.New(UnmarshalingError)
	}

	return aboutClient, nil
}

func getOperation(b []byte) (*common.Operation, error) {
	operation := &common.Operation{}

	err := json.Unmarshal(b, operation)
	if err != nil {
		log.Printf("Unmarshaling error while parsing Operation: %s\n", err.Error())
		return operation, errors.New(UnmarshalingError)
	}

	return operation, nil
}

func writeErrorResponse(conn net.Conn, s string) {
	defer conn.Close()

	errorMessage := common.Error{Message: s}
	response := common.Response{Status: "error", Error: &errorMessage}
	responseBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("Got another error while writing one error: %s", err.Error())
	}

	conn.Write(responseBytes)
	conn.Write(EOFBytes)
	conn.Close()
}

func writeOKResponse(conn net.Conn, message *json.RawMessage) error {
	response := common.Response{Status: "ok", Message: message}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("Got an error while marshaling an OK response")
		err := errors.New("Something went wrong")
		return err
	}

	conn.Write(responseBytes)
	conn.Write(EOFBytes)

	return nil
}

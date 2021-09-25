package common

import (
	"bytes"
	"encoding/json"
	"log"

	"github.com/google/uuid"
)

const (
	AboutMeOperationType   = "aboutme"
	CreateOperationType    = "create"
	SubscribeOperationType = "subscribe"
	MessageOperationType   = "message"
	ListOperationType      = "list"
)

var EOFBytes = []byte("\r\n")

// Message type describes a message being transferred between a client and a server
type Message struct {
	Conversation *Conversation `json:"conversation"`
	Sender       *Sender       `json:"sender"`
	Text         string        `json:"text"`
}

// Sender type describes a sender of a message
type Sender struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// Conversation type is where senders can send and viewers can view the messages
type Conversation struct {
	ID       uuid.UUID `json:"id"`
	Nickname string    `json:"nickname"`
}

// Error type is used to send errors
type Error struct {
	Message string `json:"message"`
}

// ClientAboutMe is a representation of the JSON message that client sends to let server know who they are
type ClientAboutMe Sender

// Operation struct is used to encapsulate general messages alongside metadata
type Operation struct {
	Type    string           `json:"type"`
	Message *json.RawMessage `json:"message"`
}

type Response struct {
	Status        string           `json:"status"`
	OperationType string           `json:"operation_type"`
	Error         *Error           `json:"error"`
	Message       *json.RawMessage `json:"message"`
}

func NewOperation() Operation {
	emptyJSON := json.RawMessage("{}")
	operation := Operation{
		Message: &emptyJSON,
	}

	return operation
}

func NewResponse() Response {
	emptyJSON := json.RawMessage("{}")
	response := Response{
		Message: &emptyJSON,
	}

	return response
}

// CheckError checks that err is not nil, and exits after a log if it isn't
func CheckError(err error) {
	if err != nil {
		// this logs to standard error and calls os.Exit(1)
		log.Fatalf("Fatal error: %s\n", err.Error())
	}
}

// CheckErrorAndLog checks that err is not nil and logs the error if it isn't
// Doesn't exit if err is not nil, but instead returns a boolean for whether err is not nil
func CheckErrorAndLog(err error) (isNotNil bool) {
	if err != nil {
		log.Printf("Error: %s\n", err.Error())
		isNotNil = true
	}

	return
}

type reader interface {
	ReadBytes(delim byte) ([]byte, error)
}

func ReadUntil(r reader, delim []byte) (returnBytes []byte, err error) {
	lastChar := delim[len(delim)-1]

	for {
		b, err := r.ReadBytes(lastChar)
		if err != nil {
			break
		}

		returnBytes = append(returnBytes, b...)

		if len(returnBytes) > len(delim) {
			bSuffix := returnBytes[len(returnBytes)-len(delim):]
			if bytes.Equal(delim, bSuffix) {
				break
			}
		}
	}

	return
}

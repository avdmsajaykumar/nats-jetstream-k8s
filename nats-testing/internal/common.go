package internal

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyz" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

// input is the payload and Description of this struct follows the payload size
type input struct {
	Id          int    `json:"id"`
	Description string `json:"description"`
}

// MessagingSystem is the common interface for publishing and subscribing to the Nats cluster
type MessagingSystem interface {
	Publish(chan<- struct{}, int)
	Subscribe(chan<- struct{})
}

// StringWithCharset returns a string with defined length of bytes
func StringWithCharset(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

// Marshal converts the input to json bytes
func Marshal(in interface{}) []byte {
	buf, _ := json.Marshal(in)
	return buf
}

// PrepareInput creates the input for publishing
func PrepareInput(length, msgSize int) [][]byte {
	in := make([][]byte, length)

	body := StringWithCharset(msgSize)
	// fmt.Printf("Body size of the message: %d:\t%s\n", len(body), body)
	path, _ := os.Getwd()

	err := os.WriteFile(fmt.Sprintf("%s/file.txt", path), []byte(body), 0777)
	if err != nil {
		os.Exit(1)
	}

	for i := 0; i < len(in); i++ {
		in[i] = Marshal(input{i, body})
	}

	return in
}

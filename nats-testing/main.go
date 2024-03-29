package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/avdmsajaykumar/nats/nats-testing/internal"
	"github.com/nats-io/nats.go"
)

var (
	msgSize, count, publishers, subscribers        int
	url, subject                                   string
	js, cleanjs, pub, sub, repeat, wqpolicy, queue bool

	Info *log.Logger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	Err  *log.Logger = log.New(os.Stdout, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
)

// Initialize flags and set default values in init function
func init() {
	flag.IntVar(&msgSize, "size", 1024, "payload size in bytes")
	flag.IntVar(&count, "count", 1, "no of messages published by each publisher")
	flag.IntVar(&publishers, "publishers", 3, "no of concurrent publishers")
	flag.IntVar(&subscribers, "subscribers", 0, "no of concurrent subscribers")
	flag.StringVar(&url, "url", "0.0.0.0:4222", "Nats endpoint")
	flag.StringVar(&subject, "subject", "foo.bar", "nats subject used")
	flag.BoolVar(&js, "js", false, "when enabled jetstream is used")
	flag.BoolVar(&cleanjs, "clean", false, "when enabled jetstream streams are removed")
	flag.BoolVar(&pub, "pub", false, "when enabled publish the messages to subject")
	flag.BoolVar(&sub, "sub", false, "when enabled subscribe to subject")
	flag.BoolVar(&repeat, "repeat", false, "continous publish/subscribe to the subject")
	flag.BoolVar(&wqpolicy, "wq", false, "use WorkQueue Policy for Jetstream")
	flag.BoolVar(&queue, "queue", false, "join all the subscribers in a queue so that only one subscriber process the message")
}

func main() {
	flag.Parse()

	// if cleanjs is enabled, remove the stream and stop from execution.
	if cleanjs {
		nc, _ := nats.Connect(url)
		internal.CleanJetstream(nc)
		return
	}

	// WorkQueuePolicy cannot have multiple consumer subscribers so we cannot create a queue to join multiple consumers
	// not stream with WorkQueuePolicy, so stop from execution if both are provided
	if wqpolicy && queue {
		Err.Println("either wq or queue should be provided, both values are supported")
		os.Exit(1)
	}

	// Initilize either the core functionality or jetstream functionality based on the arguments
	system := New()

	// subchannel is to wait till all subscriptions are closed
	subchannel := make(chan struct{})
	if sub && subscribers != 0 {
		go system.Subscribe(subchannel)
	} else {
		fmt.Println("Subscribe disabled")
		go func() { subchannel <- struct{}{} }()
	}

	// pubchannel is to wait till all publishers are closed
	pubchannel := make(chan struct{})
	if pub && publishers != 0 {
		Info.Println("Publishing initiated")
		go system.Publish(pubchannel, count)
	} else {
		Info.Println("Publish disabled")
		go func() { pubchannel <- struct{}{} }()
	}

	// wait for channels to respond
	Info.Println("waiting for channels to close")
	<-subchannel
	<-pubchannel
}

// New returns the core or jetstream struct which follows MessagingSystem interface
func New() internal.MessagingSystem {
	nc, err := nats.Connect(url)
	if err != nil {
		Err.Println("Error Connecting to Nats: " + err.Error())
		os.Exit(1)
	}
	if js {
		return internal.NewJetstream(nc, subject, msgSize, publishers, subscribers, repeat, wqpolicy, queue)
	}
	return internal.NewNatsCore(nc, subject, msgSize, publishers, subscribers, repeat)
}

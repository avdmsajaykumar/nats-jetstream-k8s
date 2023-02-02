package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/avdmsajaykumar/nats/nats-testing/internal"
	"github.com/nats-io/nats.go"
)

var (
	msgSize, count, publishers, subscribers int
	url, subject                            string
	js, cleanjs, pub, sub, repeat, wqpolicy bool
)

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
	flag.BoolVar(&wqpolicy, "wq", false, "Use WorkQueue Policy for Jetstream")
}

func main() {
	flag.Parse()

	if cleanjs {
		nc, _ := nats.Connect(url)
		internal.CleanJetstream(nc)
		return
	}
	system := New()

	subchannel := make(chan struct{})
	if sub && subscribers != 0 {
		go system.Subscribe(subchannel)
	} else {
		fmt.Println("Subscribe disabled")
		go func() { subchannel <- struct{}{} }()
	}

	pubchannel := make(chan struct{})
	if pub && publishers != 0 {
		fmt.Println("Publish")
		go system.Publish(pubchannel, count)
	} else {
		fmt.Println("Publish disabled")
		go func() { pubchannel <- struct{}{} }()
	}

	// wait for channels to respond
	fmt.Println("waiting for channels to close")
	<-subchannel
	<-pubchannel
}

func New() internal.MessagingSystem {
	nc, err := nats.Connect(url)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	if js {
		return internal.NewJetstream(nc, subject, msgSize, publishers, subscribers, repeat, wqpolicy)
	}
	return internal.NewNatsCore(nc, subject, msgSize, publishers, subscribers, repeat)
}

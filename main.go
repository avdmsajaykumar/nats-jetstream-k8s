package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	args := os.Args
	nc, _ := nats.Connect(fmt.Sprintf("localhost:%s", args[1]))

	js, _ := nc.JetStream()

	// create stream
	info, err := js.AddStream(&nats.StreamConfig{
		Name:      "DC",
		Subjects:  []string{"DC.>"},
		Retention: nats.WorkQueuePolicy,
	})
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println("Stream Name: " + info.Config.Name)

	sig := make(chan os.Signal, 1)
	exit := make(chan struct{})

	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	if args[2] == "pub" {
		go publish(js, exit)
	} else {
		go subscribe(js, info.Config.Name, args[3], sig, exit)
	}

	<-sig
	fmt.Printf("\nreceived the termination signal\n")

	exit <- struct{}{}

}

func subscribe(js nats.JetStreamContext, stream, topic string, signal chan<- os.Signal, exit <-chan struct{}) {
	subscription, err := js.PullSubscribe(fmt.Sprintf("%s.%s", stream, topic), topic, nats.BindStream("DC"))
	if err != nil {
		fmt.Printf("here %s", err.Error())
		return
	}

	for {
		select {
		case <-exit:
			fmt.Println("Stopped subscribing")
			err := subscription.Unsubscribe()
			if err != nil {
				fmt.Println("Unsubscribe error: " + err.Error())
			}
			return
		default:
			err := sub(js, subscription)
			if err != nil {
				fmt.Println("received error " + err.Error() + "\n Sending the termination signal")
				signal <- syscall.SIGINT
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func sub(js nats.JetStreamContext, subscription *nats.Subscription) error {
	msg, err := subscription.Fetch(1)
	if err != nil {
		fmt.Println("Error Fetching: " + err.Error())
		return err
	}
	fmt.Printf("%s\n", string(msg[0].Data))
	err = msg[0].Ack()
	if err != nil {
		fmt.Println("Error Here: " + err.Error())
		return errors.New("Fetch failed or empty")

	}
	return nil
}

func publish(js nats.JetStreamContext, exit <-chan struct{}) {
	i := 1
	for {
		select {
		case <-exit:
			fmt.Println("Stopped publishing")
			return

		default:
			pub(js, i)
			time.Sleep(500 * time.Millisecond)
			i++
		}
	}
}

func pub(js nats.JetStreamContext, i int) {
	input := fmt.Sprintf("%s %d: I am Batman", time.Now().Format(time.RFC3339), i)
	_, err := js.Publish("DC.batman", []byte(input))
	if err != nil {
		fmt.Println("publish error : " + err.Error())
	}

	input = fmt.Sprintf("%s %d: Kryptonite", time.Now().Format(time.RFC3339), i)
	_, err = js.Publish("DC.superman", []byte(input))
	if err != nil {
		fmt.Println("publish error: " + err.Error())
	}

	fmt.Printf("%d message published\n", i*2)
}

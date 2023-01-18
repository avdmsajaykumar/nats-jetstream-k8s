package main

import (
	"fmt"

	"github.com/nats-io/nats.go"
)

func main() {
	nc, _ := nats.Connect("localhost:4223")

	js, _ := nc.JetStream()

	// processMsg := func(msg *nats.Msg) {
	// 	fmt.Println(string(msg.Data))
	// }
	sub, err := js.PullSubscribe("john.*", "", nats.Bind("john", "c1"))
	if err != nil {
		fmt.Printf("here %s", err.Error())
		return
	}

	// fmt.Println(js.StreamInfo("john"))

	for {
		msg, err := sub.Fetch(1)
		if err != nil {
			fmt.Println("Error Here" + err.Error())
		}
		fmt.Printf("%s\n", string(msg[0].Data))
		err = msg[0].Ack()
		if err != nil {
			fmt.Println("Error Here" + err.Error())
		}
		// time.Sleep(10 * time.Second)
	}

	sub.Drain()

}

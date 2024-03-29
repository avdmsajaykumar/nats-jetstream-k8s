package internal

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

// core struct is used to test Nats Core functionality, i.e., publishing/subscribing to normal nats subjects
type core struct {
	nc                               *nats.Conn
	subject                          string
	msgSize, publishers, subscribers int
	repeat                           bool
}

// NewNatsCore returns the core struct
func NewNatsCore(nc *nats.Conn, subject string, msgSize, publishers, subscribers int, repeat bool) *core {
	Info.Println("Nats core initialized")
	return &core{nc, subject, msgSize, publishers, subscribers, repeat}
}

// Publish method publishes messages to Nats Core and supports Publish method on MessegingSystem interface
func (c *core) Publish(exit chan<- struct{}, length int) {
	now := time.Now()
	Info.Println("Time: " + now.Format(time.RFC3339))
	// payload is created before publishing to prevent additional code execution after publishing starts
	in := PrepareInput(length, c.msgSize)
	var wg sync.WaitGroup
	var totalMessages int = 0
	for i := 1; i <= c.publishers; i++ {
		wg.Add(1)

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
		go func(i int, input *[][]byte) {
			defer func() {
				Info.Printf("client %d publish completed\n", i)
				wg.Done()
			}()
			if c.repeat {
				for {
					select {
					case <-sig:
						return
					default:
						c.post(i, (*input)[0])
						totalMessages++
					}
				}

			} else {
				for j := 0; j < len(*input); j++ {
					c.post(i, (*input)[j])
				}
			}
		}(i, &in)
	}
	wg.Wait()
	// time.Sleep(1 * time.Millisecond)
	Info.Println(time.Since(now))
	Info.Printf("total messages published : %d\n", totalMessages)
	exit <- struct{}{}
}

// post adds message header and publishes the message to Nats core
func (c *core) post(i int, input []byte) {
	header := make(nats.Header)
	header.Add("publisher", fmt.Sprintf("publisher-%d", i))
	msg := &nats.Msg{
		Header:  header,
		Data:    input,
		Subject: c.subject,
	}

	err := c.nc.PublishMsg(msg)
	if err != nil {
		Err.Println("error publishing the message: " + err.Error())
	}

}

// Subscribe method creates subscribers and subscribe to the Nats Core subject and
// supports subscribe method on MessegingSystem interface
func (c *core) Subscribe(exit chan<- struct{}) {

	var wg sync.WaitGroup

	for i := 1; i <= c.subscribers; i++ {
		wg.Add(1)

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

		go func(i int) {
			defer wg.Done()
			filename := fmt.Sprintf("subscriber-%d.log", i)
			os.Remove(filename)
			sub, err := c.nc.Subscribe(c.subject, func(m *nats.Msg) {
				f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					Err.Println("error opening the file: " + err.Error())
				}
				defer f.Close()
				Info.Println(m.Header.Get("publisher"))
				// if _, err := f.WriteString(m.Header.Get("publisher") + "\t" + string(m.Data) + "\n"); err != nil {
				// 	Info.Println(err)
				// }
			})
			if err != nil {
				Err.Println("error subscribing : " + err.Error())
			}
			<-sig
			err = sub.Unsubscribe()
			if err != nil {
				Err.Println("error unsubscribing " + err.Error())
			}
			Info.Printf("subscriber %d stopped\n", i)
		}(i)
	}
	wg.Wait()
	exit <- struct{}{}

}

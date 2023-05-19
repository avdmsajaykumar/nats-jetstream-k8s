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

// streamName is used to define the name of the stream,
// also used to create other nats objects such as consumers, queuegroups
var streamName string = "TESTSTREAM"

// jetstream struct is used to test Nats jetstream functionality,
// i.e., publishing/subscribing to Jetstream enabled nats subjects
type jetstream struct {
	nc                               *nats.Conn
	js                               nats.JetStreamContext
	subject, consumerName            string
	msgSize, publishers, subscribers int
	repeat, wqPolicy, queue          bool
}

// CleanJetstream removes the stream from the cluster
func CleanJetstream(nc *nats.Conn) {
	js, err := nc.JetStream()
	if err != nil {
		os.Exit(1)
	}
	err = js.DeleteStream(streamName)
	if err != nil {
		Err.Println("error deleting stream " + err.Error())
		os.Exit(1)
	}
	Info.Println("Removed stream " + streamName)
}

// NewNatsCore returns the jetstream struct
func NewJetstream(nc *nats.Conn, subject string, msgSize, publishers, subscribers int, repeat, wqPolicy, queue bool) *jetstream {
	js, err := nc.JetStream()
	if err != nil {
		os.Exit(1)
	}
	var info *nats.StreamInfo
	consumerName := ""
	Info.Printf("server count %d\n", len(nc.DiscoveredServers()))

	// create streams

	if wqPolicy {
		// creates stream with retention policy set to WorkQueuePolicy
		info, err = js.AddStream(&nats.StreamConfig{
			Name:      streamName,
			Subjects:  []string{subject},
			Retention: nats.WorkQueuePolicy,
			Replicas:  len(nc.DiscoveredServers()),
			Storage:   nats.FileStorage,
			// MaxBytes is required to limit the streams to expand beyond the memory,
			// not setting this causes jetstream to crash
			MaxBytes: 1073741824,
		})
		if err != nil {
			Err.Println("error creating stream: " + err.Error())
			os.Exit(1)
		}
	} else if queue {
		// creates default stream with minimum config
		info, err = js.AddStream(&nats.StreamConfig{
			Name:     streamName,
			Subjects: []string{subject},
			Replicas: len(nc.DiscoveredServers()),
			MaxAge:   2 * time.Minute,
			Storage:  nats.FileStorage,
			// MaxBytes is required to limit the streams to expand beyond the memory,
			// not setting this causes jetstream to crash
			MaxBytes: 1073741824,
		})
		if err != nil {
			Err.Println("error creating stream: " + err.Error())
			os.Exit(1)
		}

	} else {
		// creates default stream with minimum config
		info, err = js.AddStream(&nats.StreamConfig{
			Name:     streamName,
			Subjects: []string{subject},
			Replicas: len(nc.DiscoveredServers()),
			MaxAge:   2 * time.Minute,
			Storage:  nats.FileStorage,
			// MaxBytes is required to limit the streams to expand beyond the memory,
			// not setting this causes jetstream to crash
			MaxBytes: 1073741824,
		})
		if err != nil {
			Err.Println("error creating stream: " + err.Error())
			os.Exit(1)
		}

	}
	// for stream with WorkQueuePolciy, creates a consumer to subscribe to the subject
	// creating the consumer while subscribing will delete the consumer when drain is called
	// so create a consumer explicitily is required
	if info.Config.Retention == nats.WorkQueuePolicy {
		consumerInfo, err := js.AddConsumer(info.Config.Name,
			&nats.ConsumerConfig{
				Durable:   info.Config.Name,
				AckPolicy: nats.AckExplicitPolicy,
			})
		if err != nil {
			Err.Println("error creating consumer: " + err.Error())
		}
		consumerName = consumerInfo.Config.Durable
	}
	Info.Println("Stream Name: " + info.Config.Name)
	Info.Println("Nats jetstream initialized")
	return &jetstream{nc, js, subject, consumerName, msgSize, publishers, subscribers, repeat, wqPolicy, queue}
}

// Publish method publishes messages to Nats Jetstream subjects
// and supports Publish method on MessegingSystem interface
func (jet *jetstream) Publish(exit chan<- struct{}, length int) {
	now := time.Now()
	Info.Println("Date: " + now.Format(time.RFC3339))
	// payload is created before publishing to prevent additional code execution after publishing starts
	in := PrepareInput(length, jet.msgSize)
	var wg sync.WaitGroup
	var totalMessages int = 0
	for i := 1; i <= jet.publishers; i++ {
		wg.Add(1)

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
		go func(i int, input *[][]byte) {
			defer func() {
				Info.Printf("client %d publish completed\n", i)
				wg.Done()
			}()

			if jet.repeat {
				count := 0
				for {
					select {
					case <-sig:
						return
					default:
						count++
						jet.post(i, count, (*input)[0])
						totalMessages++
					}
				}

			} else {
				count := 0
				for j := 0; j < len(*input); j++ {
					count++
					jet.post(i, count, (*input)[j])
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

// post adds message header and publishes the message to Nats Jetstream
func (jet *jetstream) post(i, count int, input []byte) {
	header := make(nats.Header)
	header.Add("publisher", fmt.Sprintf("publisher-%d: message %d", i, count))
	msg := &nats.Msg{
		Header:  header,
		Data:    input,
		Subject: jet.subject,
	}

	_, err := jet.js.PublishMsg(msg)
	if err != nil {
		Err.Println("error publshing message " + err.Error())
	}

}

// Subscribe method creates subscribers and subscribe to the Nats jetstream subject and
// supports subscribe method on MessegingSystem interface
func (jet *jetstream) Subscribe(exit chan<- struct{}) {

	if jet.wqPolicy {
		jet.pullSubscribe(exit)
	} else if jet.queue {
		jet.queueSubscribe(exit)
	} else {
		jet.subscribe(exit)
	}

}

// pullSubscribe method uses a pull subscribers required to listen to stream with
// retention of type WorkQueuePolicy and fetches the message on the stream subject
func (jet *jetstream) pullSubscribe(exit chan<- struct{}) {

	var wg sync.WaitGroup

	for i := 1; i <= jet.subscribers; i++ {
		wg.Add(1)

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

		go func(i int) {
			defer wg.Done()
			filename := fmt.Sprintf("subscriber-%d.log", i)
			os.Remove(filename)
			sub, err := jet.js.PullSubscribe(jet.subject, jet.consumerName, nats.BindStream(streamName))
			if err != nil {
				Err.Println("Error calling pull subscribe " + err.Error())
			}

			for {
				select {
				case <-sig:
					if err = sub.Drain(); err != nil {
						Err.Println("error draining " + err.Error())
					}

					if err = sub.Unsubscribe(); err != nil {
						Err.Println("error unsubscribing " + err.Error())
					}

					Info.Printf("subscriber %d stopped\n", i)
					return

				default:
					msg, err := sub.Fetch(1)
					if err != nil {
						Err.Println("Error Fetching: " + err.Error())
					}
					if len(msg) == 0 {
						continue
					}
					m := msg[0]
					err = m.Ack()
					if err != nil {
						Err.Println("error Ack " + err.Error())
						if err = m.Nak(); err != nil {
							Err.Println("error Nak" + err.Error())
						}

					}
					// f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
					// if err != nil {
					// 	fmt.Println(err.Error())
					// }
					Info.Println(m.Header.Get("publisher"))
					// if _, err := f.WriteString(m.Header.Get("publisher") + "\t" + string(m.Data) + "\n"); err != nil {
					// 	fmt.Println(err)
					// }
					// f.Close()
				}

			}
		}(i)
	}
	wg.Wait()
	exit <- struct{}{}
}

// queueSubscribe method adds all the subscribers to a queue group so that messages on stream subject are shared
// between the subscribers. and won't be processed by all the connected consumers
func (jet *jetstream) queueSubscribe(exit chan<- struct{}) {
	var wg sync.WaitGroup

	for i := 1; i <= jet.subscribers; i++ {
		wg.Add(1)

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

		go func(i int) {
			defer wg.Done()
			filename := fmt.Sprintf("subscriber-%d.log", i)
			os.Remove(filename)
			sub, err := jet.js.QueueSubscribe(jet.subject, streamName, func(m *nats.Msg) {
				_ = m.Ack()
				Info.Printf("processed by subscriber %d\t msg from %s\n", i, m.Header.Get("publisher"))
				// f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				// if err != nil {
				// 	fmt.Println(err.Error())
				// }
				// defer f.Close()
				// if _, err := f.WriteString(m.Header.Get("publisher") + "\t" + string(m.Data) + "\n"); err != nil {
				// 	fmt.Println(err)
				// }

			})
			if err != nil {
				Err.Println("error subscribing " + err.Error())
			}
			<-sig
			err = sub.Unsubscribe()
			if err != nil {
				Err.Println("error unsubscribing" + err.Error())
			}
			Info.Printf("subscriber %d stopped\n", i)
		}(i)
	}
	wg.Wait()
	exit <- struct{}{}

}

// subscribe method subscribes to the stream subject and reads all the messages in the stream
func (jet *jetstream) subscribe(exit chan<- struct{}) {
	var wg sync.WaitGroup

	for i := 1; i <= jet.subscribers; i++ {
		wg.Add(1)

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

		go func(i int) {
			defer wg.Done()
			filename := fmt.Sprintf("subscriber-%d.log", i)
			os.Remove(filename)
			sub, err := jet.js.Subscribe(jet.subject, func(m *nats.Msg) {
				Info.Println(m.Header.Get("publisher"))
				// f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				// if err != nil {
				// 	fmt.Println(err.Error())
				// }
				// defer f.Close()
				// if _, err := f.WriteString(m.Header.Get("publisher") + "\t" + string(m.Data) + "\n"); err != nil {
				// 	fmt.Println(err)
				// }
			})
			if err != nil {
				Err.Println("error subscribing " + err.Error())
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

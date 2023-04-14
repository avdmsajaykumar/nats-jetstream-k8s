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

var streamName string = "TESTSTREAM"

type jetstream struct {
	nc                               *nats.Conn
	js                               nats.JetStreamContext
	subject, consumerName            string
	msgSize, publishers, subscribers int
	repeat, wqPolicy, queue          bool
}

func CleanJetstream(nc *nats.Conn) {
	js, err := nc.JetStream()
	if err != nil {
		os.Exit(1)
	}
	err = js.DeleteStream(streamName)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	fmt.Println("Removed stream " + streamName)
}

func NewJetstream(nc *nats.Conn, subject string, msgSize, publishers, subscribers int, repeat, wqPolicy, queue bool) *jetstream {
	js, err := nc.JetStream()
	if err != nil {
		os.Exit(1)
	}
	var info *nats.StreamInfo
	consumerName := ""
	fmt.Printf("server count %d\n", len(nc.DiscoveredServers()))
	// create stream
	if wqPolicy {
		info, err = js.AddStream(&nats.StreamConfig{
			Name:      streamName,
			Subjects:  []string{subject},
			Retention: nats.WorkQueuePolicy,
			Replicas:  len(nc.DiscoveredServers()),
			Storage:   nats.FileStorage,
			MaxBytes:  1073741824,
		})
		if err != nil {
			fmt.Println("error: " + err.Error())
			os.Exit(1)
		}
	} else if queue {
		info, err = js.AddStream(&nats.StreamConfig{
			Name:     streamName,
			Subjects: []string{subject},
			Replicas: len(nc.DiscoveredServers()),
			MaxAge:   2 * time.Minute,
			Storage:  nats.FileStorage,
			MaxBytes: 1073741824,
		})
		if err != nil {
			fmt.Println("error: " + err.Error())
			os.Exit(1)
		}

	} else {
		info, err = js.AddStream(&nats.StreamConfig{
			Name:     streamName,
			Subjects: []string{subject},
			Replicas: len(nc.DiscoveredServers()),
			MaxAge:   2 * time.Minute,
			Storage:  nats.FileStorage,
			MaxBytes: 1073741824,
		})
		if err != nil {
			fmt.Println("error: " + err.Error())
			os.Exit(1)
		}

	}
	if info.Config.Retention == nats.WorkQueuePolicy {
		consumerInfo, err := js.AddConsumer(info.Config.Name,
			&nats.ConsumerConfig{
				Durable:   info.Config.Name,
				AckPolicy: nats.AckExplicitPolicy,
			})
		if err != nil {
			fmt.Println(err.Error())
		}
		consumerName = consumerInfo.Config.Durable
	}
	fmt.Println("Stream Name: " + info.Config.Name)
	fmt.Println("Nats jetstream initialized")
	return &jetstream{nc, js, subject, consumerName, msgSize, publishers, subscribers, repeat, wqPolicy, queue}
}

func (jet *jetstream) Publish(exit chan<- struct{}, length int) {
	now := time.Now()
	fmt.Println(now.Format(time.RFC3339))
	in := PrepareInput(length, jet.msgSize)
	var wg sync.WaitGroup
	var totalMessages int = 0
	for i := 1; i <= jet.publishers; i++ {
		wg.Add(1)

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
		go func(i int, input *[][]byte) {
			defer func() {
				fmt.Printf("client %d publish completed\n", i)
				wg.Done()
			}()

			if jet.repeat {
				for {
					select {
					case <-sig:
						return
					default:
						jet.post(i, (*input)[0])
						totalMessages++
					}
				}

			} else {
				for j := 0; j < len(*input); j++ {
					jet.post(i, (*input)[j])
				}
			}
		}(i, &in)
	}
	wg.Wait()
	// time.Sleep(1 * time.Millisecond)
	fmt.Println(time.Since(now))
	fmt.Printf("total messages published : %d\n", totalMessages)
	exit <- struct{}{}
}

func (jet *jetstream) post(i int, input []byte) {
	header := make(nats.Header)
	header.Add("publisher", fmt.Sprintf("publisher-%d", i))
	msg := &nats.Msg{
		Header:  header,
		Data:    input,
		Subject: jet.subject,
	}

	_, err := jet.js.PublishMsg(msg)
	if err != nil {
		fmt.Println(err.Error())
	}

}

func (jet *jetstream) Subscribe(exit chan<- struct{}) {

	if jet.wqPolicy {
		jet.pullSubscribe(exit)
	} else if jet.queue {
		jet.queueSubscribe(exit)
	} else {
		jet.subscribe(exit)
	}

}

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
				fmt.Println(err.Error())
			}

			for {
				select {
				case <-sig:
					if err = sub.Drain(); err != nil {
						fmt.Println(err.Error())
					}

					if err = sub.Unsubscribe(); err != nil {
						fmt.Println(err.Error())
					}

					if err != nil {
						fmt.Println(err.Error())
					}
					fmt.Printf("subscriber %d stopped\n", i)
					return

				default:
					msg, err := sub.Fetch(1)
					if err != nil {
						fmt.Println("Error Fetching: " + err.Error())
					}
					if len(msg) == 0 {
						continue
					}
					m := msg[0]
					if err := m.Ack(); err != nil {
						fmt.Println(err.Error())
					}
					if err != nil {
						fmt.Println(err.Error())
						if err = m.Nak(); err != nil {
							fmt.Println(err.Error())
						}

					}
					// f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
					// if err != nil {
					// 	fmt.Println(err.Error())
					// }
					fmt.Println(m.Header.Get("publisher"))
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
				fmt.Printf("processed by subscriber %d\t msg from %s\n", i, m.Header.Get("publisher"))
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
				fmt.Println("pull " + err.Error())
			}
			<-sig
			err = sub.Unsubscribe()
			if err != nil {
				fmt.Println(err.Error())
			}
			fmt.Printf("subscriber %d stopped\n", i)
		}(i)
	}
	wg.Wait()
	exit <- struct{}{}

}

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
				fmt.Println(m.Header.Get("publisher"))
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
				fmt.Println(err.Error())
			}
			<-sig
			err = sub.Unsubscribe()
			if err != nil {
				fmt.Println(err.Error())
			}
			fmt.Printf("subscriber %d stopped\n", i)
		}(i)
	}
	wg.Wait()
	exit <- struct{}{}
}

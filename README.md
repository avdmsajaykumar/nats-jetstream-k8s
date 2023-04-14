# Nats Jetstream


### Required tools
* helm
* Go

### Setup Nats cluster on K8S namespace
run below command to install nats
```bash
#have the ./nats/values.yaml prepared before running the below command
helm template ./nats --namespace=<namespace value> | kubectl apply -f -
```

## Test Jetstream
go into nats-testing directory and run main.go
```bash
cd nats-testing
go run ./main.go <args>
```
This takes default argument values for testing.
program take many argumets, try below command to view the available arguments

```bash
go run ./main.go --help
```
for publishing messages you need to pass ```--pub``` flag true and also needs to provide number of concurrent publishers by passing ```--publishers```  value
for subscribing messages you need to pass ```--sub``` flag true and also needs to provide number of concurrent subscribers by passing ```--subscribers``` value

for testing publishing/subscribing on jetstream pass ```--js``` flag other wise publishing and subscribing happens on Nats core where there is no persitence.  
you can test different types of streams while publishing/subscribing to jetstream subjects
not passing any of the below flags uses default stream __LimitsPolicy__.  
```--queue``` flag uses default stream type __LimitsPolicy__, however subscribers are joined together as queue so message is processed by only one subscriber.  
```--wq``` flag is used to test stream of type __WorkQueuePolicy__, where message is deleted from stream as soon as one subscriber process the message.  

__NOTE__: you cannot pass both ```--queue``` and ```--wq``` flags together.

```--clean``` will delete the stream and consumers on it.
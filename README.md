# Jetstream on minikube

### Required tools
* minikube
* docker
* helm
* Go

## Installation on Minikube.
Start the minikube server
Pull the nats server and prometheus nats exporter from docker and add to Minikube
```bash
docker pull nats:latest
synadia/prometheus-nats-exporter:latest
minikube image load nats:latest
minikube image load synadia/prometheus-nats-exporter:latest
```
Create a namespace nats-jetstream and switch to that namespace
```bash
kubectl create ns nats-jetstream
k config set-context --current --namespace=nats-jetstream
```
install the helm template
```bash
helm template --namespace=nats-jetstream ./nats >> /tmp/release.yaml
kubectl apply -f /tmp/release.yaml
```

## Test Jetstream
port forward all the nats endpoints of each instance to your local
```bash
k port-forward svc/nats-0 4222:4222 8222:8222
k port-forward svc/nats-1 4223:4222 8223:8222
k port-forward svc/nats-2 4224:4222 8224:8222
```
build test go binary
```bash
go mod tidy
go build -o nats-js
```
program take 3 argumets  
__arg 1__: port of the server which you want to connect to  
__arg 2__: 'pub' for publish && 'sub' for subsribe  
__arg 3__: topic name of the subject  (batman or superman)

```bash
# To publish messages to batman and superman topic, press ctrl^C to stop publishing
./nats-js <port> pub
# To subscribe messages to batman topic, press ctrl^C to stop subscribing if you want to stop in the middle or else program ends after reading all msgs from the topic
./nats-js <port> sub batman
# To subscribe messages to batman topic, press ctrl^C to stop subscribing if you want to stop in the middle
or else program ends after reading all msgs from the topic
./nats-js <port> sub superman
```

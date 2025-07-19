package port

type EventProducer interface {
	Product(event interface{}) error
}

type EventConsumer interface {
}

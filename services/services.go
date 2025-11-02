package services

type PubSubProvider interface {
	CreateStream(streamName string, subjects []string) error
	Publish(subjectName string, data []byte) error
	Subscribe(consumerName string, subjectName string, callBack func(msg []byte)) error
	UnSubscribe(consumerName string) error
	Close() error
}

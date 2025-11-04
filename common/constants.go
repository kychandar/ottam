package common

import "fmt"

type CachePrefix int

const (
	channelSubscription CachePrefix = iota // 0
	// Autumn                                 // 1
	// Winter                                 // 2
	// Spring                                 // 3
)

const cacheKeyFormat = "%d-%s"
const serverSubjFormat = "server-%s"

func ChannelSubscCacheKeyFormat(channel string) string {
	return fmt.Sprintf(cacheKeyFormat, channelSubscription, channel)
}

func ServerSubjFormat(server string) string {
	return fmt.Sprintf(serverSubjFormat, server)
}

const StreamNamePublisher = "publisher"

const ConsumerNameCentProcessor = "centProcessor"

type (
	ChannelName string
	NodeID      string
)

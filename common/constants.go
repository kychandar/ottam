package common

import "fmt"

type CachePrefix int

const (
	channelSubscription CachePrefix = iota // 0
)

const cacheKeyFormat = "%d-%s"
const serverSubjFormat = "server-%s"
const channelSubjFormat = "ch.%s"

func ChannelSubscCacheKeyFormat(channel string) string {
	return fmt.Sprintf(cacheKeyFormat, channelSubscription, channel)
}

func ServerSubjFormat(server string) string {
	return fmt.Sprintf(serverSubjFormat, server)
}

func ChannelSubjFormat(channel string) string {
	return fmt.Sprintf(channelSubjFormat, channel)
}

const OttamServerStreamName = "ottam-server"
const ConsumerNameFormat = "ottam-node-%s"

func ConsumerNameForNode(hostName string) string {
	return fmt.Sprintf(ConsumerNameFormat, hostName)
}

const StreamNamePublisher = "publisher"

const ConsumerNameCentProcessor = "centProcessor"

type (
	ChannelName string
	NodeID      string
)

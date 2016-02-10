package main

import (
	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/consul"
	"github.com/docker/libkv/store/etcd"
	"strings"
)

func init() {
	// Register to libkv
	consul.Register()
	etcd.Register()
}

// Splits store by types:
// [type://]url,url => [type]|nodes, [url,url]
func parseStoreUrl(rawurl string) (string, []string) {
	parts := strings.SplitN(rawurl, "://", 2)

	// nodes:port,node2:port => nodes://node1:port,node2:port
	if len(parts) == 1 {
		return "nodes", strings.Split(parts[0], ",")
	}
	return parts[0], strings.Split(parts[1], ",")
}

func NewStore(storeUrl string) (store.Store, error) {
	kv, addrs := parseStoreUrl(storeUrl)
	config := &store.Config{}

	st, err := libkv.NewStore(store.Backend(kv), addrs, config)
	if err != nil {
		return nil, err
	}

	return st, nil
}

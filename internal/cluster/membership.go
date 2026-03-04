// Copyright 2026 Scalytics, Inc.
// Copyright 2026 Mirko Kämpf
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cluster

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/hashicorp/memberlist"
)

// MembershipConfig holds configuration for cluster membership.
type MembershipConfig struct {
	NodeName      string
	BindAddr      string
	BindPort      int // gossip port (default 7946)
	AdvertiseAddr string
	AdvertisePort int
	Seeds         []string // initial peers to join
	RPCPort       int
	BoltPort      int
	HTTPPort      int
	LogOutput     io.Writer // optional; defaults to log.Writer()
}

// nodeMeta is serialized into memberlist.Node.Meta for each member.
type nodeMeta struct {
	RPCPort  int `json:"rpcPort"`
	BoltPort int `json:"boltPort"`
	HTTPPort int `json:"httpPort"`
}

// memberEvent is sent through a channel to avoid calling Members() while
// memberlist holds its internal lock (which causes deadlock on Leave).
type memberEvent struct {
	node *memberlist.Node
	kind string // "join", "leave", "update"
}

// Membership wraps hashicorp/memberlist for gossip-based cluster discovery.
type Membership struct {
	list         *memberlist.Memberlist
	self         NodeInfo
	partitionMap *PartitionMap

	mu      sync.RWMutex
	onJoin  func(NodeInfo)
	onLeave func(NodeInfo)

	eventCh  chan memberEvent
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// eventDelegate implements memberlist.EventDelegate. It sends events via a
// channel so that rebalance runs outside the memberlist lock.
type eventDelegate struct {
	eventCh chan<- memberEvent
}

func (d *eventDelegate) NotifyJoin(node *memberlist.Node) {
	select {
	case d.eventCh <- memberEvent{node: node, kind: "join"}:
	default:
	}
}

func (d *eventDelegate) NotifyLeave(node *memberlist.Node) {
	select {
	case d.eventCh <- memberEvent{node: node, kind: "leave"}:
	default:
	}
}

func (d *eventDelegate) NotifyUpdate(node *memberlist.Node) {
	select {
	case d.eventCh <- memberEvent{node: node, kind: "update"}:
	default:
	}
}

// NewMembership creates a new cluster membership backed by memberlist.
func NewMembership(cfg MembershipConfig, pm *PartitionMap) (*Membership, error) {
	if cfg.NodeName == "" {
		hostname, _ := os.Hostname()
		cfg.NodeName = hostname
	}
	if cfg.BindAddr == "" {
		cfg.BindAddr = "0.0.0.0"
	}
	if cfg.BindPort == 0 {
		cfg.BindPort = 7946
	}

	eventCh := make(chan memberEvent, 64)

	m := &Membership{
		self: NodeInfo{
			Name:     cfg.NodeName,
			Addr:     cfg.BindAddr,
			RPCPort:  cfg.RPCPort,
			BoltPort: cfg.BoltPort,
			HTTPPort: cfg.HTTPPort,
		},
		partitionMap: pm,
		eventCh:      eventCh,
		stopCh:       make(chan struct{}),
	}

	mlCfg := memberlist.DefaultLANConfig()
	mlCfg.Name = cfg.NodeName
	mlCfg.BindAddr = cfg.BindAddr
	mlCfg.BindPort = cfg.BindPort
	if cfg.AdvertiseAddr != "" {
		mlCfg.AdvertiseAddr = cfg.AdvertiseAddr
	}
	if cfg.AdvertisePort != 0 {
		mlCfg.AdvertisePort = cfg.AdvertisePort
	}

	// Encode metadata.
	meta, err := json.Marshal(nodeMeta{
		RPCPort:  cfg.RPCPort,
		BoltPort: cfg.BoltPort,
		HTTPPort: cfg.HTTPPort,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal node meta: %w", err)
	}
	mlCfg.Delegate = &metaDelegate{meta: meta}
	mlCfg.Events = &eventDelegate{eventCh: eventCh}

	logOut := cfg.LogOutput
	if logOut == nil {
		logOut = log.Writer()
	}
	mlCfg.LogOutput = logOut

	// memberlist.Create fires NotifyJoin for the local node, which writes
	// to eventCh (buffered, capacity 64). We start processEvents after
	// all fields are initialized to avoid data races on m.self and m.list.
	list, err := memberlist.Create(mlCfg)
	if err != nil {
		return nil, fmt.Errorf("create memberlist: %w", err)
	}
	m.list = list
	m.self.Addr = list.LocalNode().Addr.String()

	// Initial rebalance with self.
	m.partitionMap.Rebalance([]NodeInfo{m.self})

	// Now start the event processor — any events buffered during Create
	// will be drained and trigger a rebalance with the current member list.
	m.wg.Add(1)
	go m.processEvents()

	return m, nil
}

// processEvents drains the event channel and triggers rebalances outside the
// memberlist internal lock.
func (m *Membership) processEvents() {
	defer m.wg.Done()
	for {
		select {
		case <-m.stopCh:
			return
		case ev := <-m.eventCh:
			// Rebalance from current members.
			if m.list != nil {
				m.partitionMap.Rebalance(m.Members())
			}
			info := nodeInfoFromMember(ev.node)
			m.mu.RLock()
			onJoin := m.onJoin
			onLeave := m.onLeave
			m.mu.RUnlock()
			switch ev.kind {
			case "join":
				if onJoin != nil {
					onJoin(info)
				}
			case "leave":
				if onLeave != nil {
					onLeave(info)
				}
			}
		}
	}
}

// Join attempts to join the cluster by contacting the given seed addresses.
func (m *Membership) Join(seeds []string) error {
	if len(seeds) == 0 {
		return nil
	}
	n, err := m.list.Join(seeds)
	if err != nil {
		return fmt.Errorf("join cluster: %w (contacted %d nodes)", err, n)
	}
	return nil
}

// Leave gracefully leaves the cluster and stops the event processor.
// It is safe to call multiple times.
func (m *Membership) Leave() error {
	var err error
	m.stopOnce.Do(func() {
		err = m.list.Leave(0)
		close(m.stopCh)
		m.wg.Wait()
		_ = m.list.Shutdown()
	})
	return err
}

// Members returns all current cluster members as NodeInfo.
func (m *Membership) Members() []NodeInfo {
	if m.list == nil {
		return []NodeInfo{m.self}
	}
	members := m.list.Members()
	result := make([]NodeInfo, 0, len(members))
	for _, member := range members {
		result = append(result, nodeInfoFromMember(member))
	}
	return result
}

// Self returns this node's NodeInfo.
func (m *Membership) Self() NodeInfo {
	return m.self
}

// OnJoin registers a callback invoked when a node joins the cluster.
func (m *Membership) OnJoin(fn func(NodeInfo)) {
	m.mu.Lock()
	m.onJoin = fn
	m.mu.Unlock()
}

// OnLeave registers a callback invoked when a node leaves the cluster.
func (m *Membership) OnLeave(fn func(NodeInfo)) {
	m.mu.Lock()
	m.onLeave = fn
	m.mu.Unlock()
}

// nodeInfoFromMember extracts NodeInfo from a memberlist.Node.
func nodeInfoFromMember(node *memberlist.Node) NodeInfo {
	info := NodeInfo{
		Name: node.Name,
		Addr: node.Addr.String(),
	}
	var meta nodeMeta
	if err := json.Unmarshal(node.Meta, &meta); err == nil {
		info.RPCPort = meta.RPCPort
		info.BoltPort = meta.BoltPort
		info.HTTPPort = meta.HTTPPort
	}
	return info
}

// metaDelegate provides node metadata to memberlist.
type metaDelegate struct {
	meta []byte
}

func (d *metaDelegate) NodeMeta(int) []byte             { return d.meta }
func (d *metaDelegate) NotifyMsg([]byte)                {}
func (d *metaDelegate) GetBroadcasts(int, int) [][]byte { return nil }
func (d *metaDelegate) LocalState(bool) []byte          { return nil }
func (d *metaDelegate) MergeRemoteState([]byte, bool)   {}

/*
Copyright 2018-2022 Mailgun Technologies Inc

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gubernator

import (
	"github.com/mailgun/holster/v4/syncutil"
)

type RegionPeerPicker interface {
	GetClients(string) ([]*PeerClient, error)
	GetByPeerInfo(PeerInfo) *PeerClient
	Pickers() map[string]PeerPicker
	Peers() []*PeerClient
	Add(*PeerClient)
	New() RegionPeerPicker
}

// RegionPicker encapsulates pickers for a set of regions
type RegionPicker struct {
	*ReplicatedConsistentHash

	// A map of all the pickers by region
	regions map[string]PeerPicker
	// The implementation of picker we will use for each region
	conf     BehaviorConfig
	wg       syncutil.WaitGroup
	reqQueue chan *RateLimitReq
}

func NewRegionPicker(fn HashString64) *RegionPicker {
	rp := &RegionPicker{
		regions:                  make(map[string]PeerPicker),
		reqQueue:                 make(chan *RateLimitReq, 0),
		ReplicatedConsistentHash: NewReplicatedConsistentHash(fn, defaultReplicas),
	}
	return rp
}

func (rp *RegionPicker) New() RegionPeerPicker {
	hash := rp.ReplicatedConsistentHash.New().(*ReplicatedConsistentHash)
	return &RegionPicker{
		regions:                  make(map[string]PeerPicker),
		reqQueue:                 make(chan *RateLimitReq, 0),
		ReplicatedConsistentHash: hash,
	}
}

// GetClients returns all the PeerClients that match this key in all regions
func (rp *RegionPicker) GetClients(key string) ([]*PeerClient, error) {
	result := make([]*PeerClient, len(rp.regions))
	var i int
	for _, picker := range rp.regions {
		peer, err := picker.Get(key)
		if err != nil {
			return nil, err
		}
		result[i] = peer
		i++
	}
	return result, nil
}

// GetByPeerInfo returns the first PeerClient the PeerInfo.HasKey() matches
func (rp *RegionPicker) GetByPeerInfo(info PeerInfo) *PeerClient {
	for _, picker := range rp.regions {
		if client := picker.GetByPeerInfo(info); client != nil {
			return client
		}
	}
	return nil
}

// Pickers returns a map of each region and its respective PeerPicker
func (rp *RegionPicker) Pickers() map[string]PeerPicker {
	return rp.regions
}

func (rp *RegionPicker) Peers() []*PeerClient {
	var peers []*PeerClient

	for _, picker := range rp.regions {
		for _, peer := range picker.Peers() {
			peers = append(peers, peer)
		}
	}

	return peers
}

func (rp *RegionPicker) Add(peer *PeerClient) {
	picker, ok := rp.regions[peer.Info().DataCenter]
	if !ok {
		picker = rp.ReplicatedConsistentHash.New()
		rp.regions[peer.Info().DataCenter] = picker
	}
	picker.Add(peer)
}

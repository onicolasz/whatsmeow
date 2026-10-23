// Copyright (c) 2026 DKW System
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow/types"
)

func newCleanupTestClient() *Client {
	return &Client{
		userDevicesCache:            make(map[types.JID]deviceCache),
		groupCache:                  make(map[types.JID]*groupMetaCache),
		messageRetries:              make(map[string]int),
		incomingRetryRequestCounter: make(map[incomingRetryKey]int),
		sessionRecreateHistory:      make(map[types.JID]time.Time),
		appStateKeyRequests:         make(map[string]time.Time),
	}
}

func TestCleanupUserDevicesCache(t *testing.T) {
	cli := newCleanupTestClient()
	now := time.Now()
	idle := types.NewJID("111", types.DefaultUserServer)
	hot := types.NewJID("222", types.DefaultUserServer)
	fresh := types.NewJID("333", types.DefaultUserServer)
	cli.userDevicesCache[idle] = deviceCache{devices: []types.JID{idle}, lastUsed: now.Add(-2 * userDevicesCacheIdleTTL)}
	cli.userDevicesCache[hot] = deviceCache{devices: []types.JID{hot}, lastUsed: now.Add(-time.Minute)}
	cli.userDevicesCache[fresh] = deviceCache{devices: []types.JID{fresh}}

	cli.unlockedCleanupUserDevicesCache()

	if _, ok := cli.userDevicesCache[idle]; ok {
		t.Error("idle entry should have been evicted")
	}
	if _, ok := cli.userDevicesCache[hot]; !ok {
		t.Error("recently used entry should have been kept")
	}
	if cached, ok := cli.userDevicesCache[fresh]; !ok || cached.lastUsed.IsZero() {
		t.Error("entry written since the last sweep should be kept and get its idle timer started")
	}

	// Throttled: a second sweep within the interval doesn't touch anything
	cli.userDevicesCache[idle] = deviceCache{devices: []types.JID{idle}, lastUsed: now.Add(-2 * userDevicesCacheIdleTTL)}
	cli.unlockedCleanupUserDevicesCache()
	if _, ok := cli.userDevicesCache[idle]; !ok {
		t.Error("sweep should be throttled by cacheCleanupInterval")
	}
}

func TestCleanupGroupCache(t *testing.T) {
	cli := newCleanupTestClient()
	now := time.Now()
	idle := types.NewJID("111", types.GroupServer)
	hot := types.NewJID("222", types.GroupServer)
	fresh := types.NewJID("333", types.GroupServer)
	cli.groupCache[idle] = &groupMetaCache{lastUsed: now.Add(-2 * groupCacheIdleTTL)}
	cli.groupCache[hot] = &groupMetaCache{lastUsed: now.Add(-time.Minute)}
	cli.groupCache[fresh] = &groupMetaCache{}

	cli.unlockedCleanupGroupCache()

	if _, ok := cli.groupCache[idle]; ok {
		t.Error("idle group should have been evicted")
	}
	if _, ok := cli.groupCache[hot]; !ok {
		t.Error("recently used group should have been kept")
	}
	if cached, ok := cli.groupCache[fresh]; !ok || cached.lastUsed.IsZero() {
		t.Error("group cached since the last sweep should be kept and get its idle timer started")
	}
}

func TestResetRetryCounters(t *testing.T) {
	cli := newCleanupTestClient()
	cli.unlockedResetMessageRetries()
	cli.messageRetries["msg"] = 3
	cli.unlockedResetMessageRetries()
	if cli.messageRetries["msg"] != 3 {
		t.Error("message retries should not be reset within the interval")
	}
	cli.lastMessageRetriesReset = time.Now().Add(-2 * retryCounterResetInterval)
	cli.unlockedResetMessageRetries()
	if len(cli.messageRetries) != 0 {
		t.Error("message retries should be reset after the interval")
	}

	key := incomingRetryKey{types.NewJID("111", types.DefaultUserServer), "msg"}
	cli.unlockedResetIncomingRetryRequestCounter()
	cli.incomingRetryRequestCounter[key] = 5
	cli.lastIncomingRetryRequestCounterReset = time.Now().Add(-2 * retryCounterResetInterval)
	cli.unlockedResetIncomingRetryRequestCounter()
	if len(cli.incomingRetryRequestCounter) != 0 {
		t.Error("incoming retry counter should be reset after the interval")
	}
}

func TestCleanupTimestampMaps(t *testing.T) {
	cli := newCleanupTestClient()
	now := time.Now()
	old := types.NewJID("111", types.DefaultUserServer)
	recent := types.NewJID("222", types.DefaultUserServer)
	cli.sessionRecreateHistory[old] = now.Add(-2 * recreateSessionTimeout)
	cli.sessionRecreateHistory[recent] = now.Add(-time.Minute)
	cli.appStateKeyRequests["old"] = now.Add(-2 * appStateKeyRequestTTL)
	cli.appStateKeyRequests["recent"] = now.Add(-time.Hour)

	cli.unlockedCleanupSessionRecreateHistory()
	cli.unlockedCleanupAppStateKeyRequests()

	if _, ok := cli.sessionRecreateHistory[old]; ok {
		t.Error("expired session recreate entry should have been removed")
	}
	if _, ok := cli.sessionRecreateHistory[recent]; !ok {
		t.Error("recent session recreate entry should have been kept")
	}
	if _, ok := cli.appStateKeyRequests["old"]; ok {
		t.Error("expired app state key request should have been removed")
	}
	if _, ok := cli.appStateKeyRequests["recent"]; !ok {
		t.Error("recent app state key request should have been kept")
	}
}

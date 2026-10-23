// Copyright (c) 2026 DKW System
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"time"
)

// Long-running clients (hundreds of sessions in one process, weeks of uptime) would otherwise keep
// every device list, group and retry counter they ever touched in memory. The caches below are swept
// lazily while their lock is already held, at most once per cacheCleanupInterval. Evicted entries are
// simply fetched again from the server or the store when they're needed.
const (
	cacheCleanupInterval      = 10 * time.Minute
	userDevicesCacheIdleTTL   = 1 * time.Hour
	groupCacheIdleTTL         = 1 * time.Hour
	retryCounterResetInterval = 1 * time.Hour
	appStateKeyRequestTTL     = 48 * time.Hour
)

// compactMap copies m into a map sized for its current length. Go maps never release bucket memory
// when entries are deleted, so a cache that was once big stays big unless it's reallocated.
func compactMap[K comparable, V any](m map[K]V) map[K]V {
	out := make(map[K]V, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// unlockedCleanupUserDevicesCache must be called with userDevicesCacheLock held.
func (cli *Client) unlockedCleanupUserDevicesCache() {
	now := time.Now()
	if now.Sub(cli.lastUserDevicesCacheCleanup) < cacheCleanupInterval {
		return
	}
	cli.lastUserDevicesCacheCleanup = now
	deleted := 0
	for jid, cached := range cli.userDevicesCache {
		if cached.lastUsed.IsZero() {
			// Written but not read since the last sweep, start its idle timer now
			cached.lastUsed = now
			cli.userDevicesCache[jid] = cached
		} else if now.Sub(cached.lastUsed) > userDevicesCacheIdleTTL {
			delete(cli.userDevicesCache, jid)
			deleted++
		}
	}
	if deleted > 0 {
		cli.userDevicesCache = compactMap(cli.userDevicesCache)
	}
}

// unlockedCleanupGroupCache must be called with groupCacheLock held.
func (cli *Client) unlockedCleanupGroupCache() {
	now := time.Now()
	if now.Sub(cli.lastGroupCacheCleanup) < cacheCleanupInterval {
		return
	}
	cli.lastGroupCacheCleanup = now
	deleted := 0
	for jid, cached := range cli.groupCache {
		if cached.lastUsed.IsZero() {
			cached.lastUsed = now
		} else if now.Sub(cached.lastUsed) > groupCacheIdleTTL {
			delete(cli.groupCache, jid)
			deleted++
		}
	}
	if deleted > 0 {
		cli.groupCache = compactMap(cli.groupCache)
	}
}

// unlockedResetMessageRetries must be called with messageRetriesLock held.
// The counters only matter while a message is actively being retried, which happens within minutes.
func (cli *Client) unlockedResetMessageRetries() {
	if time.Since(cli.lastMessageRetriesReset) < retryCounterResetInterval {
		return
	}
	cli.lastMessageRetriesReset = time.Now()
	cli.messageRetries = make(map[string]int)
}

// unlockedResetIncomingRetryRequestCounter must be called with incomingRetryRequestCounterLock held.
func (cli *Client) unlockedResetIncomingRetryRequestCounter() {
	if time.Since(cli.lastIncomingRetryRequestCounterReset) < retryCounterResetInterval {
		return
	}
	cli.lastIncomingRetryRequestCounterReset = time.Now()
	cli.incomingRetryRequestCounter = make(map[incomingRetryKey]int)
}

// unlockedCleanupSessionRecreateHistory must be called with sessionRecreateHistoryLock held.
// Entries older than recreateSessionTimeout behave exactly like missing ones, so dropping them is lossless.
func (cli *Client) unlockedCleanupSessionRecreateHistory() {
	now := time.Now()
	if now.Sub(cli.lastSessionRecreateHistoryCleanup) < cacheCleanupInterval {
		return
	}
	cli.lastSessionRecreateHistoryCleanup = now
	deleted := 0
	for jid, ts := range cli.sessionRecreateHistory {
		if now.Sub(ts) > recreateSessionTimeout {
			delete(cli.sessionRecreateHistory, jid)
			deleted++
		}
	}
	if deleted > 0 {
		cli.sessionRecreateHistory = compactMap(cli.sessionRecreateHistory)
	}
}

// unlockedCleanupAppStateKeyRequests must be called with appStateKeyRequestsLock held (write lock).
func (cli *Client) unlockedCleanupAppStateKeyRequests() {
	now := time.Now()
	if now.Sub(cli.lastAppStateKeyRequestsCleanup) < cacheCleanupInterval {
		return
	}
	cli.lastAppStateKeyRequestsCleanup = now
	deleted := 0
	for keyID, ts := range cli.appStateKeyRequests {
		if now.Sub(ts) > appStateKeyRequestTTL {
			delete(cli.appStateKeyRequests, keyID)
			deleted++
		}
	}
	if deleted > 0 {
		cli.appStateKeyRequests = compactMap(cli.appStateKeyRequests)
	}
}

package Bot

import (
	"sync"
	"testing"
	"time"
)

func newTestService() *BotService {
	return &BotService{chats: map[int64]*chatState{}}
}

func TestTouchMessageThrottles(t *testing.T) {
	bs := newTestService()

	if bs.touchMessage(1) {
		t.Error("the very first message must never be throttled")
	}
	if !bs.touchMessage(1) {
		t.Error("a message right after the previous one should be throttled")
	}

	// A different chat has its own clock.
	if bs.touchMessage(2) {
		t.Error("another chat must not be affected")
	}

	// Once the cooldown has passed, messages flow again.
	bs.chats[1].lastMessage = time.Now().Add(-2 * AntiFloodCooldown)
	if bs.touchMessage(1) {
		t.Error("message after the cooldown should pass")
	}
}

// Inline buttons must not throttle a message the user types straight after.
func TestTouchActionDoesNotThrottleMessages(t *testing.T) {
	bs := newTestService()

	bs.touchAction(1)
	bs.touchAction(1)
	if bs.touchMessage(1) {
		t.Error("callbacks must not feed the anti-flood clock")
	}
	if bs.chats[1].lastAction.IsZero() {
		t.Error("callbacks should still count as activity for the janitor")
	}
}

func TestTransientTracking(t *testing.T) {
	bs := newTestService()

	bs.trackTransient(1, 10, 11)
	bs.trackTransient(1, 12)

	got := bs.takeTransient(1)
	if len(got) != 3 {
		t.Fatalf("took %v, want 3 ids", got)
	}
	if again := bs.takeTransient(1); len(again) != 0 {
		t.Errorf("ids should be handed over only once, got %v", again)
	}
}

func TestPruneIdleChats(t *testing.T) {
	bs := newTestService()

	bs.touchMessage(1) // active
	bs.touchMessage(2) // will be aged out
	bs.chats[2].lastAction = time.Now().Add(-2 * idleTTL)

	// A busy chat must survive even if its clock looks old.
	bs.touchMessage(3)
	bs.chats[3].lastAction = time.Now().Add(-2 * idleTTL)
	bs.chats[3].lock.Lock()

	bs.pruneIdleChats()

	if _, ok := bs.chats[1]; !ok {
		t.Error("active chat was pruned")
	}
	if _, ok := bs.chats[2]; ok {
		t.Error("idle chat should have been pruned")
	}
	if _, ok := bs.chats[3]; !ok {
		t.Error("a chat whose lock is held must never be pruned")
	}
	bs.chats[3].lock.Unlock()
}

// Guards the maps against concurrent access; run with -race.
func TestChatStateConcurrentAccess(t *testing.T) {
	bs := newTestService()

	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		chatID := int64(i % 5) // deliberate contention on the same chats
		go func(chatID int64) {
			defer wg.Done()

			bs.touchMessage(chatID)
			bs.touchAction(chatID)

			state := bs.chat(chatID)
			state.lock.Lock()
			bs.trackTransient(chatID, 1, 2)
			bs.takeTransient(chatID)
			state.lock.Unlock()

			bs.pruneIdleChats()
		}(chatID)
	}
	wg.Wait()
}

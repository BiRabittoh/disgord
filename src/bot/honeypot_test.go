package bot

import (
	"sync"
	"testing"
)

func TestHoneypotStateManagement(t *testing.T) {
	// Verify HoneypotState structure field types
	state := &HoneypotState{
		ChannelID: "123",
		MessageID: "456",
		BanCount:  0,
	}

	if state.ChannelID != "123" {
		t.Errorf("expected ChannelID to be '123', got '%s'", state.ChannelID)
	}

	// Test thread-safe read/write concurrency to Honeypots map
	honeypots := make(map[string]*HoneypotState)
	var mu sync.RWMutex

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			mu.Lock()
			honeypots["guild-1"] = &HoneypotState{
				ChannelID: "channel-1",
				MessageID: "msg-1",
				BanCount:  id,
			}
			mu.Unlock()
		}(i)

		go func() {
			defer wg.Done()
			mu.RLock()
			_ = honeypots["guild-1"]
			mu.RUnlock()
		}()
	}
	wg.Wait()
}

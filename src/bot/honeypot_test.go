package bot

import (
	"os"
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

func TestHoneypotPersistence(t *testing.T) {
	// Ensure we start without a file
	os.Remove(HoneypotsFile)
	defer os.Remove(HoneypotsFile)

	// Create a dummy BotService
	bs := &BotService{
		Honeypots: make(map[string]*HoneypotState),
	}

	// Set some test honeypots
	bs.Honeypots["guild-123"] = &HoneypotState{
		ChannelID: "channel-abc",
		MessageID: "msg-xyz",
		BanCount:  42,
	}

	// Save
	bs.SaveHoneypots()

	// Create another dummy BotService and load
	bs2 := &BotService{
		Honeypots: make(map[string]*HoneypotState),
	}
	bs2.LoadHoneypots()

	state, ok := bs2.Honeypots["guild-123"]
	if !ok {
		t.Fatalf("expected guild-123 to be loaded")
	}

	if state.ChannelID != "channel-abc" || state.MessageID != "msg-xyz" || state.BanCount != 42 {
		t.Errorf("loaded honeypot state is incorrect: %+v", state)
	}
}

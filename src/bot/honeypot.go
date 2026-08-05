package bot

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bwmarrin/discordgo"
)

const HoneypotsFile = "honeypots.json"

type HoneypotState struct {
	ChannelID string `json:"channel_id"`
	MessageID string `json:"message_id"`
	BanCount  int    `json:"ban_count"`
}

func (bs *BotService) SaveHoneypots() {
	bs.HoneypotsMu.RLock()
	defer bs.HoneypotsMu.RUnlock()

	data, err := json.MarshalIndent(bs.Honeypots, "", "  ")
	if err != nil {
		bs.logger.Error("failed to marshal honeypots config", "error", err)
		return
	}

	err = os.WriteFile(HoneypotsFile, data, 0644)
	if err != nil {
		bs.logger.Error("failed to write honeypots config file", "error", err)
	}
}

func (bs *BotService) LoadHoneypots() {
	bs.HoneypotsMu.Lock()
	defer bs.HoneypotsMu.Unlock()

	data, err := os.ReadFile(HoneypotsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		bs.logger.Error("failed to read honeypots config file", "error", err)
		return
	}

	err = json.Unmarshal(data, &bs.Honeypots)
	if err != nil {
		bs.logger.Error("failed to unmarshal honeypots config", "error", err)
	}
}

func (bs *BotService) IsAdmin(guildID, userID, channelID string) bool {
	guild, err := bs.US.Session.State.Guild(guildID)
	if err != nil {
		return false
	}
	if guild.OwnerID == userID {
		return true
	}
	perms, err := bs.US.Session.State.UserChannelPermissions(userID, channelID)
	if err != nil {
		return false
	}
	return perms&discordgo.PermissionAdministrator != 0
}

// CheckHoneypot checks if a message was sent in a honeypot channel and handles banning/updating if so.
// It returns true if the message was in a honeypot and the user was banned (or if the handler should stop processing).
func (bs *BotService) CheckHoneypot(s *discordgo.Session, m *discordgo.MessageCreate) bool {
	if m.GuildID == "" {
		return false
	}

	bs.HoneypotsMu.RLock()
	hp, exists := bs.Honeypots[m.GuildID]
	bs.HoneypotsMu.RUnlock()

	if !exists || hp == nil || hp.ChannelID != m.ChannelID {
		return false
	}

	// Message was sent in the honeypot channel!
	// Check if the author is a bot admin
	isAdmin := bs.IsAdmin(m.GuildID, m.Author.ID, m.ChannelID)
	if isAdmin {
		// Bot admins are allowed to type in the honeypot channel
		return false
	}

	// User is NOT an admin. Instantly permaban them and delete messages from last 24h (1 day).
	err := s.GuildBanCreateWithReason(m.GuildID, m.Author.ID, "Triggered honeypot channel", 1)
	if err != nil {
		bs.logger.Error("failed to ban user after triggering honeypot", "guild_id", m.GuildID, "user_id", m.Author.ID, "error", err)
		return true // Still intercept so we don't process it further
	}

	bs.logger.Info("User banned by honeypot", "guild_id", m.GuildID, "user_id", m.Author.ID)

	// Increment BanCount and edit initial honeypot message
	bs.HoneypotsMu.Lock()
	hp.BanCount++
	banCount := hp.BanCount
	msgID := hp.MessageID
	bs.HoneypotsMu.Unlock()

	bs.SaveHoneypots()

	if msgID != "" {
		newContent := fmt.Sprintf("**%d** people were banned because they wrote here.", banCount)
		_, err := s.ChannelMessageEdit(m.ChannelID, msgID, newContent)
		if err != nil {
			bs.logger.Error("failed to edit honeypot message", "channel_id", m.ChannelID, "message_id", msgID, "error", err)
		}
	}

	return true
}

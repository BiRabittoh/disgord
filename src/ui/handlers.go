package ui

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/birabittoh/disgord/src/bot"
	"github.com/birabittoh/disgord/src/globals"
)

func (ui *UIService) indexHandler(w http.ResponseWriter, r *http.Request) {
	var b bytes.Buffer
	err := ui.indexTemplate.Execute(&b, map[string]any{
		"botName":    ui.botName,
		"inviteLink": ui.inviteLink,
		"commitID":   globals.CommitID,
	})
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write(b.Bytes())
}

func (ui *UIService) guildsHandler(w http.ResponseWriter, r *http.Request) {
	if !ui.IsBotEnabled() {
		jsonSuccess(w, []any{})
		return
	}

	jsonSuccess(w, ui.us.Session.State.Guilds)
}

func (ui *UIService) queuesHandler(w http.ResponseWriter, r *http.Request) {
	if !ui.IsBotEnabled() || ui.bs.MS == nil {
		jsonSuccess(w, []any{})
		return
	}

	response := []map[string]any{}
	for guildID, queue := range ui.bs.MS.Queues {
		response = append(response, map[string]any{
			"guild_id":   guildID,
			"channel_id": queue.VoiceChannelID(),
			"tracks":     queue.Tracks(), // first track is currently playing
		})
	}
	jsonSuccess(w, response)
}

func (ui *UIService) queuesCommandsHandler(w http.ResponseWriter, r *http.Request) {
	jsonSuccess(w, ui.queueCmds)
}

func (ui *UIService) queuesCommandHandler(w http.ResponseWriter, r *http.Request) {
	if ui.bs.MS == nil {
		jsonError(w, "Music service is disabled", http.StatusServiceUnavailable)
		return
	}

	guildID := r.PathValue("guild_id")
	if guildID == "" {
		jsonError(w, "Guild ID is required", http.StatusBadRequest)
		return
	}

	var payload QueueCommandPayload
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		jsonError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	command, ok := ui.validQueueCmds[payload.Command]
	if !ok {
		jsonError(w, "Invalid command", http.StatusBadRequest)
		return
	}

	err = command(guildID, payload)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (ui *UIService) guildLeaveHandler(w http.ResponseWriter, r *http.Request) {
	guildID := r.PathValue("id")
	if guildID == "" {
		jsonError(w, "Guild ID is required", http.StatusBadRequest)
		return
	}

	err := ui.us.Session.GuildLeave(guildID)
	if err != nil {
		jsonError(w, "Failed to leave guild", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Command Handlers

func (ui *UIService) handleQueuePlay(guildID string, payload QueueCommandPayload) error {
	if payload.VoiceChannelID == "" {
		return errors.New("VoiceChannelID is required for play command")
	}

	_, _, err := ui.bs.MS.PlayToVC(payload.Args, payload.VoiceChannelID, guildID)
	return err
}

func (ui *UIService) handleQueueClear(guildID string, payload QueueCommandPayload) error {
	queue := ui.bs.MS.GetQueue(guildID)
	if queue == nil {
		return errors.New("no active queue for this guild")
	}
	queue.Clear()
	return nil
}

func (ui *UIService) handleQueueSkip(guildID string, payload QueueCommandPayload) error {
	queue := ui.bs.MS.GetQueue(guildID)
	if queue == nil {
		return errors.New("no active queue for this guild")
	}
	return queue.PlayNext(ui.bs.MS, true)
}

func (ui *UIService) handleQueueStop(guildID string, payload QueueCommandPayload) error {
	ui.bs.MS.DeleteQueue(guildID)
	return nil
}

func (ui *UIService) getBotStateHandler(w http.ResponseWriter, r *http.Request) {
	jsonSuccess(w, EnabledPayload{Enabled: ui.IsBotEnabled()})
}

func (ui *UIService) postBotStateHandler(w http.ResponseWriter, r *http.Request) {
	var payload EnabledPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		jsonError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if payload.Enabled {
		if !ui.IsBotEnabled() {
			var err error
			ui.bs, err = bot.NewBotService(ui.us.Config)
			if err != nil {
				jsonError(w, "Failed to create bot service: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
	} else {
		if ui.IsBotEnabled() {
			ui.bs.Stop()
			ui.bs = nil
		}
	}

	jsonSuccess(w, EnabledPayload{Enabled: ui.IsBotEnabled()})
}

func (ui *UIService) healthzHandler(w http.ResponseWriter, r *http.Request) {
	// A nil bs means the bot was intentionally disabled via the UI: report healthy
	// so the container isn't restarted. Otherwise the gateway must be alive.
	if ui.bs != nil && !ui.bs.IsConnected() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type HoneypotPayload struct {
	GuildID   string `json:"guild_id"`
	ChannelID string `json:"channel_id"`
}

func (ui *UIService) getHoneypotsHandler(w http.ResponseWriter, r *http.Request) {
	if !ui.IsBotEnabled() {
		jsonSuccess(w, map[string]any{})
		return
	}
	ui.bs.HoneypotsMu.RLock()
	defer ui.bs.HoneypotsMu.RUnlock()
	jsonSuccess(w, ui.bs.Honeypots)
}

func (ui *UIService) postHoneypotsHandler(w http.ResponseWriter, r *http.Request) {
	if !ui.IsBotEnabled() {
		jsonError(w, "Bot is disabled", http.StatusServiceUnavailable)
		return
	}

	var payload HoneypotPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		jsonError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if payload.GuildID == "" || payload.ChannelID == "" {
		jsonError(w, "Guild ID and Channel ID are required", http.StatusBadRequest)
		return
	}

	// Send the initial message to that channel
	content := "**🍯 Honeypot Channel 🍯**\n\nI have banned **0** people who wrote here."
	msg, err := ui.us.Session.ChannelMessageSend(payload.ChannelID, content)
	if err != nil {
		jsonError(w, "Failed to send initial honeypot message to the channel. Make sure the bot has access to that channel: "+err.Error(), http.StatusInternalServerError)
		return
	}

	ui.bs.HoneypotsMu.Lock()
	ui.bs.Honeypots[payload.GuildID] = &bot.HoneypotState{
		ChannelID: payload.ChannelID,
		MessageID: msg.ID,
		BanCount:  0,
	}
	ui.bs.HoneypotsMu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

func (ui *UIService) deleteHoneypotHandler(w http.ResponseWriter, r *http.Request) {
	if !ui.IsBotEnabled() {
		jsonError(w, "Bot is disabled", http.StatusServiceUnavailable)
		return
	}

	guildID := r.PathValue("guild_id")
	if guildID == "" {
		jsonError(w, "Guild ID is required", http.StatusBadRequest)
		return
	}

	ui.bs.HoneypotsMu.Lock()
	delete(ui.bs.Honeypots, guildID)
	ui.bs.HoneypotsMu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

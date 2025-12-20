package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/susu3304/nkmzbot/internal/transcribe"
	"github.com/susu3304/nkmzbot/internal/voice"
)

func HandleJoin(s *discordgo.Session, i *discordgo.InteractionCreate, vm *voice.Manager) {
	guildID := i.GuildID
	userID := i.Member.User.ID

	// Find the channel the user is in
	guild, err := s.State.Guild(guildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to find guild info",
			},
		})
		return
	}

	var channelID string
	for _, vs := range guild.VoiceStates {
		if vs.UserID == userID {
			channelID = vs.ChannelID
			break
		}
	}

	if channelID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You must be in a voice channel to use this command",
			},
		})
		return
	}

	// Check if already in a session
	if vm.GetSession(guildID) != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Already recording in this guild",
			},
		})
		return
	}

	// Join the channel
	vc, err := s.ChannelVoiceJoin(guildID, channelID, false, false)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Failed to join voice channel: %v", err),
			},
		})
		return
	}

	// Start recording session
	session := voice.NewSession(guildID, vc)
	vm.AddSession(guildID, session)
	session.StartRecording()

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Joined <#%s> and started recording!", channelID),
		},
	})
}

func HandleLeave(s *discordgo.Session, i *discordgo.InteractionCreate, vm *voice.Manager, tr *transcribe.Client) {
	guildID := i.GuildID
	session := vm.GetSession(guildID)
	if session == nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Not currently in a voice session",
			},
		})
		return
	}

	// Defer response
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	files := session.StopRecording()
	vm.RemoveSession(guildID)
	// Disconnect can fail if already disconnected, but usually fine
	session.Conn.Disconnect()

	if len(files) == 0 {
		msg := "Left voice channel. No audio recorded."
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &msg,
		})
		return
	}

	var result strings.Builder
	result.WriteString("Left voice channel. Transcriptions:\n")

	for _, file := range files {
		// Transcribe
		text, err := tr.Transcribe(context.Background(), file)
		if err != nil {
			result.WriteString(fmt.Sprintf("- File %s: Error: %v\n", file, err))
		} else {
			result.WriteString(fmt.Sprintf("- %s\n", text))
		}
	}

	content := result.String()
	if len(content) > 2000 {
		content = content[:1997] + "..."
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	})
}

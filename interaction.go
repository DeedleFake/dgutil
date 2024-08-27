package dgutil

import "github.com/bwmarrin/discordgo"

// SetupResponse sets up the interaction for a deferred response.
func SetupResponse(dg *discordgo.Session, i *discordgo.Interaction) error {
	return dg.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Working on it...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

// UpdateResponse sets the content of a deferred interaction response
// as set up by [setupResponse].
func UpdateResponse(dg *discordgo.Session, i *discordgo.Interaction, content string) error {
	_, err := dg.InteractionResponseEdit(i, &discordgo.WebhookEdit{
		Content: &content,
	})
	return err
}

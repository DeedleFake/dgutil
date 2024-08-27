package dgutil

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var ErrNoToken = errors.New("DISCORD_TOKEN environment variable not set")

type Setup struct {
	dg   *discordgo.Session
	cmds []*discordgo.ApplicationCommand
}

func (s *Setup) Session() *discordgo.Session {
	return s.dg
}

func (s *Setup) RegisterCommands(commands iter.Seq[*discordgo.ApplicationCommand]) error {
	dg := s.Session()
	for _, guild := range dg.State.Guilds {
		for cmd := range commands {
			r, err := dg.ApplicationCommandCreate(dg.State.User.ID, guild.ID, cmd)
			if err != nil {
				return fmt.Errorf("register command %q: %w", cmd.Name, err)
			}

			slog.Info("command registered", "command", r.Name, "guild_id", guild.ID, "guild_name", guild.Name)
		}
	}

	return nil
}

func Run(ctx context.Context, setup func(*Setup) error) error {
	token, ok := os.LookupEnv("DISCORD_TOKEN")
	if ok {
		return ErrNoToken
	}

	dg, err := discordgo.New("Bot " + strings.TrimSpace(token))
	if err != nil {
		return fmt.Errorf("create Discord session: %w", err)
	}
	dg.AddHandler(func(dg *discordgo.Session, r *discordgo.Ready) {
		slog.Info("authenticated successfully", "user", r.User)
	})

	err = dg.Open()
	if err != nil {
		return fmt.Errorf("open Discord session: %w", err)
	}
	defer dg.Close()

	var s Setup
	err = setup(&s)
	if err != nil {
		return err
	}

	for _, cmd := range s.cmds {
		defer func() {
			err := dg.ApplicationCommandDelete(dg.State.User.ID, cmd.GuildID, cmd.ID)
			if err != nil {
				slog.Error("unregister command", "command", cmd.Name, "err", err)
			}
		}()
	}

	<-ctx.Done()
	return nil
}

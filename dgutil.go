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

// ErrNoToken is returned by [Run] if the token could not be found.
var ErrNoToken = errors.New("DISCORD_TOKEN environment variable not set")

// Setup is a wrapper that helps with setting up the configuration for
// a bot.
type Setup struct {
	dg   *discordgo.Session
	cmds []*discordgo.ApplicationCommand
}

// Session returns the underlying [discordgo.Session].
func (s *Setup) Session() *discordgo.Session {
	return s.dg
}

// RegisterCommands registers a set of commands with the underlying
// [discordgo.Session] for every guild that the bot is in. When the
// bot exits, the commands will be automatically unregistered.
func (s *Setup) RegisterCommands(commands iter.Seq[*discordgo.ApplicationCommand]) error {
	dg := s.Session()
	for _, guild := range dg.State.Guilds {
		for cmd := range commands {
			r, err := dg.ApplicationCommandCreate(dg.State.User.ID, guild.ID, cmd)
			if err != nil {
				return fmt.Errorf("register command %q: %w", cmd.Name, err)
			}

			slog.Info("command registered", "command", r.Name, "guild_id", guild.ID, "guild_name", guild.Name)
			s.cmds = append(s.cmds, r)
		}
	}

	return nil
}

// AddHandlerWithContext adds an event handler to a
// [discordgo.Session], but includes an extra context argument. The
// context provided to the handler itself will be a child of ctx that
// will be canceled when the handler returns.
func AddHandlerWithContext[T any](ctx context.Context, dg *discordgo.Session, h func(context.Context, *discordgo.Session, T)) func() {
	return dg.AddHandler(func(dg *discordgo.Session, ev T) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		h(ctx, dg, ev)
	})
}

// Run runs a Discord bot. It pulls the auth token from the
// $DISCORD_TOKEN environment variable, connects to Discord's API,
// then calls the provided setup function. When the provided context
// is canceled, it will exit, cleaning up whatever it did while
// setting up.
func Run(ctx context.Context, setup func(*Setup) error) error {
	token, ok := os.LookupEnv("DISCORD_TOKEN")
	if !ok {
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

	s := Setup{
		dg: dg,
	}
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

package dgutil

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
)

// ErrNoToken is returned by [Run] if the token could not be found.
var ErrNoToken = errors.New("DISCORD_TOKEN environment variable not set")

// Bot represents a Discord bot.
type Bot struct {
	// Commands is a list of commands to register in the various guilds
	// that the bot is in. The will be automatically unregistered when
	// [Run] returns. If the bot joins a new server, the commands will
	// be automatically registered there, too.
	Commands iter.Seq[*discordgo.ApplicationCommand]

	once    sync.Once
	session *discordgo.Session
	err     error
}

func (bot *Bot) init() {
	bot.once.Do(func() {
		token, ok := os.LookupEnv("DISCORD_TOKEN")
		if !ok {
			bot.err = ErrNoToken
			return
		}

		dg, err := discordgo.New("Bot " + strings.TrimSpace(token))
		if err != nil {
			bot.err = fmt.Errorf("create Discord session: %w", err)
			return
		}
		bot.session = dg
	})
}

// Session returns the underlying session, initializing one if
// necessary.
func (bot *Bot) Session() (*discordgo.Session, error) {
	bot.init()
	return bot.session, bot.err
}

// Run runs a Discord bot. It pulls the auth token from the
// $DISCORD_TOKEN environment variable and connects to Discord's API.
// This function blocks until the provided context is canceled, at
// which point it will clean up and then return.
//
// It is invalid to call this function twice.
func (bot *Bot) Run(ctx context.Context) error {
	bot.init()
	if bot.err != nil {
		return bot.err
	}
	dg := bot.session

	dg.AddHandler(func(dg *discordgo.Session, r *discordgo.Ready) {
		slog.Info("authenticated successfully", "user", r.User)
	})

	var registered []*discordgo.ApplicationCommand
	var registeredm sync.Mutex
	dg.AddHandler(func(dg *discordgo.Session, g *discordgo.GuildCreate) {
		slog := slog.With("guild_id", g.ID, "guild_name", g.Name)
		slog.Info("entered guild")

		dg.State.RLock()
		userID := dg.State.User.ID
		dg.State.RUnlock()

		registeredm.Lock()
		defer registeredm.Unlock()

		for cmd := range bot.Commands {
			slog := slog.With("command", cmd.Name)

			reg, err := dg.ApplicationCommandCreate(userID, g.ID, cmd)
			if err != nil {
				slog.Error("failed to register command", "err", err)
				continue
			}

			slog.Info("command registered")
			registered = append(registered, reg)
		}
	})

	err := dg.Open()
	if err != nil {
		return fmt.Errorf("open Discord session: %w", err)
	}
	defer dg.Close()

	defer func() {
		dg.State.RLock()
		userID := dg.State.User.ID
		dg.State.RUnlock()

		registeredm.Lock()
		defer registeredm.Unlock()
		for _, cmd := range registered {
			slog := slog.With("guild_id", cmd.GuildID, "command", cmd.Name)
			err := dg.ApplicationCommandDelete(userID, cmd.GuildID, cmd.ID)
			if err != nil {
				slog.Error("failed to unregister command", "err", err)
				continue
			}
			slog.Info("command unregistered")
		}
	}()

	<-ctx.Done()
	return nil
}

// AddHandler adds an event handler to a [discordgo.Session], but
// includes an extra context argument and panic protection. The
// context provided to the handler itself will be a child of ctx that
// will be canceled when the handler returns.
func AddHandler[T any](ctx context.Context, dg *discordgo.Session, h func(context.Context, *discordgo.Session, T)) func() {
	return dg.AddHandler(func(dg *discordgo.Session, ev T) {
		defer func() {
			r := recover()
			if r != nil {
				slog.Error("recovered from panic in handler", "value", r)
				debug.PrintStack()
			}
		}()

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		h(ctx, dg, ev)
	})
}
